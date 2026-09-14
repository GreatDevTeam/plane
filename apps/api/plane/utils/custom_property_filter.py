# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

"""Filtering work items by a user defined property.

A custom property filter is keyed ``property_<uuid>__<lookup>``, carrying the same
``property_`` prefix the display property toggles do (``WORK_ITEM_PROPERTY_DISPLAY_KEY_PREFIX``
on the web side). The value itself lives in a
typed column of ``issue_property_values``, so which column a lookup reads is decided
by the property's ``property_type`` rather than by the shape of the incoming value.

Every condition becomes its own subquery over ``IssuePropertyValue``. Expressing it as
a join instead would be wrong: Django requires all conditions of a single ``filter()``
call against a multi valued relation to be satisfied by the same related row, so two
custom property conditions would ask one value row to belong to two properties at
once and match nothing.
"""

import re
import uuid
from datetime import datetime, time, timedelta

from django.db.models import Q
from django.utils import timezone
from django.utils.dateparse import parse_date, parse_datetime
from rest_framework.exceptions import ValidationError as DRFValidationError

_UUID_PATTERN = r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"

# `property_<uuid>__<lookup>`
CUSTOM_PROPERTY_FILTER_RE = re.compile(
    rf"^property_(?P<property_id>{_UUID_PATTERN})__(?P<lookup>exact|in|range|icontains)$"
)

# `property_<uuid>` — the legacy query parameter path carries no lookup
CUSTOM_PROPERTY_PARAM_RE = re.compile(rf"^property_(?P<property_id>{_UUID_PATTERN})$")

# The typed column each property type stores its value in
COLUMN_BY_PROPERTY_TYPE = {
    "TEXT": "value_text",
    "URL": "value_text",
    "EMAIL": "value_text",
    "FILE": "value_text",
    "DECIMAL": "value_decimal",
    "BOOLEAN": "value_boolean",
    "DATETIME": "value_datetime",
    "OPTION": "value_uuid",
    "RELATION": "value_uuid",
}

_TEXT_COLUMN = "value_text"

_TRUTHY = {"true", "1", "yes"}
_FALSY = {"false", "0", "no"}


def parse_custom_property_key(key):
    """Return ``(property_id, lookup)`` for a custom property filter key, else ``None``."""
    match = CUSTOM_PROPERTY_FILTER_RE.match(key)
    if not match:
        return None
    return match.group("property_id"), match.group("lookup")


def parse_custom_property_param(key):
    """Return the property id of a legacy ``property_<uuid>`` query parameter, else ``None``."""
    match = CUSTOM_PROPERTY_PARAM_RE.match(key)
    return match.group("property_id") if match else None


def _invalid(message):
    raise DRFValidationError({"message": message, "code": "invalid_custom_property_filter"})


def _coerce(value, property_type, property_id):
    """Coerce one incoming string into the type of the column it is compared against."""
    if value is None:
        _invalid(f"Filtering on property '{property_id}' requires a value")

    column = COLUMN_BY_PROPERTY_TYPE[property_type]

    if column == _TEXT_COLUMN:
        return str(value)

    if column == "value_decimal":
        try:
            return float(value)
        except (TypeError, ValueError):
            _invalid(f"'{value}' is not a number, which property '{property_id}' expects")

    if column == "value_boolean":
        normalized = str(value).strip().lower()
        if normalized in _TRUTHY:
            return True
        if normalized in _FALSY:
            return False
        _invalid(f"'{value}' is not a boolean, which property '{property_id}' expects")

    if column == "value_datetime":
        # narrowed to a calendar day, so a plain date and a full timestamp both work
        parsed = parse_date(str(value))
        if parsed is None:
            parsed_datetime = parse_datetime(str(value))
            parsed = parsed_datetime.date() if parsed_datetime else None
        if parsed is None:
            _invalid(f"'{value}' is not a date, which property '{property_id}' expects")
        return parsed

    try:
        return uuid.UUID(str(value))
    except (TypeError, ValueError):
        _invalid(f"'{value}' is not an id, which property '{property_id}' expects")


def _as_list(value):
    if isinstance(value, (list, tuple)):
        return list(value)
    return [value]


def _day_bounds(first, last):
    """The half open timestamp range covering the calendar days ``first`` to ``last``.

    A datetime property is filtered by calendar day — an exact timestamp match is
    never what a date picker means. Comparing the raw column against these bounds
    keeps the lookup on ``ipv_property_datetime_idx``, which a ``__date`` lookup
    would not: that wraps the column in a cast and the index no longer applies.
    """
    start = timezone.make_aware(datetime.combine(first, time.min))
    end = timezone.make_aware(datetime.combine(last + timedelta(days=1), time.min))
    return start, end


def _lookup_condition(property_type, lookup, values, property_id):
    """The condition on the value row, as a ``Q`` over the property's own column."""
    column = COLUMN_BY_PROPERTY_TYPE[property_type]
    is_datetime = column == "value_datetime"

    if lookup == "icontains":
        if column != _TEXT_COLUMN:
            _invalid(f"Property '{property_id}' does not support the 'icontains' filter")
        return Q(**{f"{column}__icontains": values[0]})

    def day(first, last):
        start, end = _day_bounds(first, last)
        return Q(**{f"{column}__gte": start, f"{column}__lt": end})

    if lookup == "range":
        if len(values) != 2:
            _invalid(f"Filtering property '{property_id}' by range needs exactly two values")
        if is_datetime:
            return day(values[0], values[1])
        return Q(**{f"{column}__range": (values[0], values[1])})

    if lookup == "in":
        if is_datetime:
            # one calendar day per value, so the days are ORed rather than listed
            days = Q()
            for value in values:
                days |= day(value, value)
            return days
        return Q(**{f"{column}__in": values})

    if is_datetime:
        return day(values[0], values[0])

    return Q(**{column: values[0]})


def custom_property_issue_ids(property_id, lookup, value):
    """The ids of the work items whose value of ``property_id`` satisfies ``lookup``.

    Returned as an unevaluated queryset so the caller can nest it in a ``pk__in``.
    """
    # imported here rather than at module scope: the legacy filter path is imported
    # from `plane.db.models.view`, i.e. while the model package is still loading
    from plane.db.models import IssueProperty, IssuePropertyValue

    property_type = IssueProperty.objects.filter(pk=property_id).values_list("property_type", flat=True).first()
    if property_type is None:
        _invalid(f"Property '{property_id}' does not exist")
    if property_type not in COLUMN_BY_PROPERTY_TYPE:
        _invalid(f"Property '{property_id}' cannot be filtered on")

    values = [_coerce(item, property_type, property_id) for item in _as_list(value)]
    if not values:
        _invalid(f"Filtering on property '{property_id}' requires a value")

    return IssuePropertyValue.objects.filter(
        _lookup_condition(property_type, lookup, values, property_id),
        property_id=property_id,
        issue_id__isnull=False,
    ).values("issue_id")

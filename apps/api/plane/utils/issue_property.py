# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

"""Coercion and validation of work item property values.

Values arrive from the API as plain strings (one per row, several for a multi
valued property) and are stored in the typed column that matches the property's
``property_type``, so later phases can filter and order on an index.
"""

# Python imports
import uuid

# Django imports
from django.core.exceptions import ValidationError
from django.core.validators import EmailValidator, URLValidator
from django.utils.dateparse import parse_datetime
from django.utils.timezone import is_naive, make_aware

# Module imports
from plane.db.models import Issue, IssuePropertyOption, PropertyTypeEnum, RelationTypeEnum, WorkspaceMember

# The typed column each property type stores its value in
VALUE_FIELD_BY_PROPERTY_TYPE = {
    PropertyTypeEnum.TEXT: "value_text",
    PropertyTypeEnum.URL: "value_text",
    PropertyTypeEnum.EMAIL: "value_text",
    PropertyTypeEnum.FILE: "value_text",
    PropertyTypeEnum.DECIMAL: "value_decimal",
    PropertyTypeEnum.BOOLEAN: "value_boolean",
    PropertyTypeEnum.DATETIME: "value_datetime",
    PropertyTypeEnum.OPTION: "value_uuid",
    PropertyTypeEnum.RELATION: "value_uuid",
}

BOOLEAN_TRUE = {"true", "1", "yes", "on"}
BOOLEAN_FALSE = {"false", "0", "no", "off"}


class PropertyValueError(Exception):
    """Raised when a submitted value does not fit its property."""

    def __init__(self, property_id, message):
        self.property_id = str(property_id)
        self.message = message
        super().__init__(message)


def value_field_for(property_type):
    return VALUE_FIELD_BY_PROPERTY_TYPE.get(property_type, "value_text")


def _as_str(value):
    return value if isinstance(value, str) else str(value)


def _coerce_text(issue_property, value):
    text = _as_str(value).strip()
    if not text:
        return None

    settings = issue_property.settings or {}
    max_length = settings.get("max_length")
    if isinstance(max_length, int) and len(text) > max_length:
        raise PropertyValueError(issue_property.id, f"Value cannot be longer than {max_length} characters")

    if issue_property.property_type == PropertyTypeEnum.URL:
        try:
            URLValidator()(text)
        except ValidationError:
            raise PropertyValueError(issue_property.id, "Value is not a valid URL")
    elif issue_property.property_type == PropertyTypeEnum.EMAIL:
        try:
            EmailValidator()(text)
        except ValidationError:
            raise PropertyValueError(issue_property.id, "Value is not a valid email address")

    return text


def _coerce_decimal(issue_property, value):
    try:
        number = float(value)
    except (TypeError, ValueError):
        raise PropertyValueError(issue_property.id, "Value is not a number")

    settings = issue_property.settings or {}
    minimum, maximum = settings.get("min"), settings.get("max")
    if isinstance(minimum, (int, float)) and number < minimum:
        raise PropertyValueError(issue_property.id, f"Value cannot be smaller than {minimum}")
    if isinstance(maximum, (int, float)) and number > maximum:
        raise PropertyValueError(issue_property.id, f"Value cannot be greater than {maximum}")

    return number


def _coerce_boolean(issue_property, value):
    if isinstance(value, bool):
        return value

    text = _as_str(value).strip().lower()
    if text in BOOLEAN_TRUE:
        return True
    if text in BOOLEAN_FALSE:
        return False
    raise PropertyValueError(issue_property.id, "Value is not a boolean")


def _coerce_datetime(issue_property, value):
    parsed = parse_datetime(_as_str(value).strip())
    if parsed is None:
        raise PropertyValueError(issue_property.id, "Value is not a valid ISO 8601 datetime")
    return make_aware(parsed) if is_naive(parsed) else parsed


def _coerce_uuid(issue_property, value):
    try:
        return uuid.UUID(_as_str(value).strip())
    except (TypeError, ValueError):
        raise PropertyValueError(issue_property.id, "Value is not a valid id")


COERCERS = {
    PropertyTypeEnum.TEXT: _coerce_text,
    PropertyTypeEnum.URL: _coerce_text,
    PropertyTypeEnum.EMAIL: _coerce_text,
    PropertyTypeEnum.FILE: _coerce_text,
    PropertyTypeEnum.DECIMAL: _coerce_decimal,
    PropertyTypeEnum.BOOLEAN: _coerce_boolean,
    PropertyTypeEnum.DATETIME: _coerce_datetime,
    PropertyTypeEnum.OPTION: _coerce_uuid,
    PropertyTypeEnum.RELATION: _coerce_uuid,
}


def _check_referenced_ids(issue_property, ids, project):
    """Make sure OPTION / RELATION values point at something that exists."""
    if not ids:
        return

    if issue_property.property_type == PropertyTypeEnum.OPTION:
        known = set(
            IssuePropertyOption.objects.filter(property_id=issue_property.id, pk__in=ids).values_list("pk", flat=True)
        )
        missing = [value for value in ids if value not in known]
        if missing:
            raise PropertyValueError(issue_property.id, "Value is not an option of this property")
        return

    if issue_property.relation_type == RelationTypeEnum.USER:
        known = set(
            WorkspaceMember.objects.filter(
                workspace_id=project.workspace_id, member_id__in=ids, is_active=True
            ).values_list("member_id", flat=True)
        )
        missing = [value for value in ids if value not in known]
        if missing:
            raise PropertyValueError(issue_property.id, "Value is not a member of this workspace")
        return

    if issue_property.relation_type == RelationTypeEnum.ISSUE:
        known = set(Issue.objects.filter(project_id=project.id, pk__in=ids).values_list("pk", flat=True))
        missing = [value for value in ids if value not in known]
        if missing:
            raise PropertyValueError(issue_property.id, "Value is not a work item of this project")
        return

    raise PropertyValueError(issue_property.id, "The property does not declare what it relates to")


def coerce_values(issue_property, values, project):
    """Turn the submitted values of one property into typed column values.

    Returns a list of values for ``value_field_for(property.property_type)``.
    Raises ``PropertyValueError`` when the property does not accept them.
    """
    if not issue_property.is_active:
        raise PropertyValueError(issue_property.id, "The property is not active")

    raw_values = values if isinstance(values, list) else [values]
    coercer = COERCERS[issue_property.property_type]

    coerced = []
    for raw_value in raw_values:
        if raw_value is None:
            continue
        coerced_value = coercer(issue_property, raw_value)
        if coerced_value is None:
            continue
        if coerced_value not in coerced:
            coerced.append(coerced_value)

    if not issue_property.is_multi and len(coerced) > 1:
        raise PropertyValueError(issue_property.id, "The property accepts a single value")

    if issue_property.is_required and not coerced:
        raise PropertyValueError(issue_property.id, "The property is required")

    if issue_property.property_type in (PropertyTypeEnum.OPTION, PropertyTypeEnum.RELATION):
        _check_referenced_ids(issue_property, coerced, project)

    return coerced


def serialize_value(issue_property, value_row):
    """Render one stored row back as the string the API hands out."""
    value = getattr(value_row, value_field_for(issue_property.property_type))
    if value is None:
        return None
    if issue_property.property_type == PropertyTypeEnum.DATETIME:
        return value.isoformat()
    if issue_property.property_type == PropertyTypeEnum.BOOLEAN:
        return value
    if issue_property.property_type == PropertyTypeEnum.DECIMAL:
        return value
    return str(value)

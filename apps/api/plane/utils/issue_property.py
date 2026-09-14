# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

"""Coercion and validation of work item property values.

Values arrive from the API as plain strings (one per row, several for a multi
valued property) and are stored in the typed column that matches the property's
``property_type``, so later phases can filter and order on an index.
"""

# Python imports
import re
import uuid
from collections import defaultdict

# Django imports
from django.core.exceptions import ValidationError
from django.core.validators import EmailValidator, URLValidator
from django.db import transaction
from django.utils import timezone
from django.utils.dateparse import parse_datetime
from django.utils.timezone import is_naive, make_aware

# Module imports
from plane.db.models import (
    Issue,
    IssueProperty,
    IssuePropertyActionEnum,
    IssuePropertyActivity,
    IssuePropertyOption,
    IssuePropertyValue,
    PropertyTypeEnum,
    RelationTypeEnum,
    WorkspaceMember,
)

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


# A work item identifier as the export writes it, e.g. `PROJ-12`
ISSUE_IDENTIFIER_RE = re.compile(r"^(?P<identifier>.+)-(?P<sequence_id>\d+)$")


def _label_map(issue_property, values):
    """``{stored value: label}`` for one property's OPTION / RELATION ids, in one query.

    Everything else is its own label, so the map comes back empty and the caller
    falls through to the stored value.
    """
    ids = [value for value in values if value is not None]
    if not ids or issue_property.property_type not in (PropertyTypeEnum.OPTION, PropertyTypeEnum.RELATION):
        return {}

    if issue_property.property_type == PropertyTypeEnum.OPTION:
        return {
            str(pk): name
            for pk, name in IssuePropertyOption.objects.filter(
                property_id=issue_property.id, pk__in=ids
            ).values_list("pk", "name")
        }

    if issue_property.relation_type == RelationTypeEnum.USER:
        return {
            str(member.member_id): member.member.display_name or member.member.email
            for member in WorkspaceMember.objects.filter(member_id__in=ids).select_related("member")
        }

    if issue_property.relation_type == RelationTypeEnum.ISSUE:
        return {
            str(pk): f"{identifier}-{sequence_id}"
            for pk, identifier, sequence_id in Issue.objects.filter(pk__in=ids).values_list(
                "pk", "project__identifier", "sequence_id"
            )
        }

    return {}


def display_values(issue_property, values):
    """Render stored values as the text the activity feed and the export show.

    An option, a member and a related work item are rendered by name rather than by
    id. The activity feed keeps the text it resolved to at the time, so renaming an
    option later does not rewrite what the feed says happened.
    """
    labels = _label_map(issue_property, values)
    return [labels.get(str(value), value) if value is not None else value for value in values]


def resolve_reference_values(issue_property, values, project):
    """Map the labels the export writes back onto the ids ``coerce_values`` takes.

    The export renders an option as its name, a member as their display name and a
    relation as ``PROJ-12``, so the public API accepts those back alongside raw ids —
    otherwise an exported work item could not be imported again. A value that is
    already an id, or that resolves to nothing, is handed on untouched and fails
    validation in ``coerce_values`` as it normally would.
    """
    if issue_property.property_type not in (PropertyTypeEnum.OPTION, PropertyTypeEnum.RELATION):
        return values

    raw_values = values if isinstance(values, list) else [values]
    labels = []
    for value in raw_values:
        if value is None:
            continue
        text = _as_str(value).strip()
        try:
            uuid.UUID(text)
        except ValueError:
            labels.append(text)

    if not labels:
        return values

    lookup = _id_by_label(issue_property, labels, project)
    return [lookup.get(_as_str(value).strip(), value) if value is not None else value for value in raw_values]


def _id_by_label(issue_property, labels, project):
    """``{label: id}`` for the labels of one property, in one query."""
    if issue_property.property_type == PropertyTypeEnum.OPTION:
        return {
            name: str(pk)
            for pk, name in IssuePropertyOption.objects.filter(
                property_id=issue_property.id, name__in=labels
            ).values_list("pk", "name")
        }

    if issue_property.relation_type == RelationTypeEnum.USER:
        lookup = {}
        for member in WorkspaceMember.objects.filter(
            workspace_id=project.workspace_id, is_active=True
        ).select_related("member"):
            for label in (member.member.email, member.member.display_name):
                if label in labels:
                    lookup.setdefault(label, str(member.member_id))
        return lookup

    if issue_property.relation_type == RelationTypeEnum.ISSUE:
        sequence_ids = {}
        for label in labels:
            match = ISSUE_IDENTIFIER_RE.match(label)
            if match and match.group("identifier") == project.identifier:
                sequence_ids[int(match.group("sequence_id"))] = label
        if not sequence_ids:
            return {}
        return {
            sequence_ids[sequence_id]: str(pk)
            for pk, sequence_id in Issue.objects.filter(
                project_id=project.id, sequence_id__in=sequence_ids.keys()
            ).values_list("pk", "sequence_id")
        }

    return {}


def property_values_index(issue_ids, key="display_name", as_labels=True):
    """``{issue id: {property key: [value, …]}}`` for a batch of work items.

    Resolved in a handful of queries rather than one round trip per work item — the
    export runs over a whole workspace. ``key`` picks the property attribute the
    values are filed under (``display_name`` for the export, ``name`` — the stable
    api key — for the webhook payload), and ``as_labels`` whether option and relation
    ids are rendered as their names.
    """
    # A queryset is left alone so that it stays a subquery — the export hands one in
    # rather than pulling every work item id of a workspace into memory
    if isinstance(issue_ids, (list, tuple, set)) and not issue_ids:
        return {}

    rows_by_property = defaultdict(list)
    for row in (
        IssuePropertyValue.objects.filter(issue_id__in=issue_ids).select_related("property").order_by("created_at")
    ):
        rows_by_property[row.property_id].append(row)

    index = defaultdict(lambda: defaultdict(list))
    for rows in rows_by_property.values():
        issue_property = rows[0].property
        values = [(row.issue_id, serialize_value(issue_property, row)) for row in rows]
        labels = _label_map(issue_property, [value for _, value in values]) if as_labels else {}
        for issue_id, value in values:
            if value is None:
                continue
            index[str(issue_id)][getattr(issue_property, key)].append(labels.get(str(value), value))

    return {issue_id: dict(values) for issue_id, values in index.items()}


def properties_of(issue_type_ids):
    """The properties of the given work item types, in the order they are shown in."""
    return IssueProperty.objects.filter(issue_type_id__in=issue_type_ids).order_by("sort_order", "created_at")


def values_map(owner_ids, properties, owner_field="issue"):
    """``{owner_id: {property_id: [value, …]}}`` for the given work items or drafts."""
    properties_by_id = {issue_property.id: issue_property for issue_property in properties}

    values = defaultdict(lambda: defaultdict(list))
    rows = IssuePropertyValue.objects.filter(
        **{f"{owner_field}_id__in": owner_ids}, property_id__in=properties_by_id.keys()
    ).order_by("created_at")

    for row in rows:
        issue_property = properties_by_id[row.property_id]
        value = serialize_value(issue_property, row)
        if value is not None:
            values[str(getattr(row, f"{owner_field}_id"))][str(row.property_id)].append(value)

    # A work item with no value for a property still has to carry the empty
    # list, otherwise the client cannot tell "not loaded" from "not set"
    return {
        str(owner_id): {
            str(issue_property.id): values[str(owner_id)].get(str(issue_property.id), [])
            for issue_property in properties
        }
        for owner_id in owner_ids
    }


def action_for(old_values, new_values):
    if not old_values:
        return IssuePropertyActionEnum.CREATED
    if not new_values:
        return IssuePropertyActionEnum.DELETED
    return IssuePropertyActionEnum.UPDATED


def is_reference(issue_property):
    """Whether the property's values are ids of something else."""
    return issue_property.property_type in (PropertyTypeEnum.OPTION, PropertyTypeEnum.RELATION)


def replace_property_values(issue, property_values, actor, owner_field="issue", accept_labels=False):
    """Replace the values of the submitted properties on one work item or draft.

    Properties that are not in the payload are left alone, so a partial save from the
    detail sidebar does not wipe the rest of the form. Returns ``(values, errors)``;
    ``errors`` is ``{property_id: message}`` and nothing is written when it is set.

    A property is addressed by id, or — with ``accept_labels``, which the public API
    turns on so that an exported work item can be imported back — by its ``name``,
    with option and relation values given as the labels the export writes.
    """
    properties = {}
    for issue_property in properties_of([issue.type_id]):
        properties[str(issue_property.id)] = issue_property
        if accept_labels:
            properties.setdefault(issue_property.name, issue_property)

    coerced = {}
    errors = {}
    for property_key, raw_values in property_values.items():
        issue_property = properties.get(str(property_key))
        if issue_property is None:
            errors[str(property_key)] = "The property does not belong to the work item type"
            continue
        if accept_labels:
            raw_values = resolve_reference_values(issue_property, raw_values, issue.project)
        try:
            coerced[str(issue_property.id)] = coerce_values(issue_property, raw_values, issue.project)
        except PropertyValueError as error:
            errors[error.property_id] = error.message

    if errors:
        return None, errors

    epoch = int(timezone.now().timestamp())
    owner_filter = {f"{owner_field}_id": issue.id}
    with transaction.atomic():
        for property_id, values in coerced.items():
            issue_property = properties[property_id]
            existing = list(IssuePropertyValue.objects.filter(**owner_filter, property_id=property_id))
            old_values = [serialize_value(issue_property, row) for row in existing]
            new_values = [
                serialize_value(
                    issue_property,
                    IssuePropertyValue(**{value_field_for(issue_property.property_type): value}),
                )
                for value in values
            ]

            if old_values == new_values:
                continue

            IssuePropertyValue.objects.filter(**owner_filter, property_id=property_id).delete()
            IssuePropertyValue.objects.bulk_create(
                [
                    IssuePropertyValue(
                        **owner_filter,
                        property_id=property_id,
                        project_id=issue.project_id,
                        workspace_id=issue.workspace_id,
                        created_by=actor,
                        **{value_field_for(issue_property.property_type): value},
                    )
                    for value in values
                ],
                batch_size=100,
            )

            # A draft has no activity feed, and `IssuePropertyActivity.issue`
            # cannot point at one — the values are recorded when it converts
            if owner_field != "issue":
                continue

            # The feed shows what the value read as when it was set — an option or a
            # member is recorded by name, not by the id that renders as nothing once
            # it is deleted
            IssuePropertyActivity.objects.create(
                issue_id=issue.id,
                property_id=issue_property.id,
                project_id=issue.project_id,
                workspace_id=issue.workspace_id,
                actor=actor,
                action=action_for(old_values, new_values),
                old_value=", ".join(str(value) for value in display_values(issue_property, old_values)) or None,
                new_value=", ".join(str(value) for value in display_values(issue_property, new_values)) or None,
                old_identifier=old_values[0] if len(old_values) == 1 and is_reference(issue_property) else None,
                new_identifier=new_values[0] if len(new_values) == 1 and is_reference(issue_property) else None,
                epoch=epoch,
            )

    return values_map([issue.id], properties_of([issue.type_id]), owner_field)[str(issue.id)], None

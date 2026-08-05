# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Django imports
from django.conf import settings
from django.db import models
from django.db.models import Q

# Module imports
from .base import BaseModel
from .project import ProjectBaseModel


class PropertyTypeEnum(models.TextChoices):
    """The kind of value a work item property holds."""

    TEXT = "TEXT", "Text"
    DECIMAL = "DECIMAL", "Decimal"
    OPTION = "OPTION", "Option"
    BOOLEAN = "BOOLEAN", "Boolean"
    DATETIME = "DATETIME", "Datetime"
    RELATION = "RELATION", "Relation"
    URL = "URL", "URL"
    EMAIL = "EMAIL", "Email"
    FILE = "FILE", "File"


class RelationTypeEnum(models.TextChoices):
    """What a ``RELATION`` property points at."""

    USER = "USER", "User"
    ISSUE = "ISSUE", "Issue"


class IssuePropertyActionEnum(models.TextChoices):
    CREATED = "created", "Created"
    UPDATED = "updated", "Updated"
    DELETED = "deleted", "Deleted"


class IssueProperty(BaseModel):
    """A user defined field on a work item type.

    Properties hang off ``IssueType`` rather than off the project, matching the
    ``Issue.type`` FK — a work item gets the property set of its type.
    """

    workspace = models.ForeignKey("db.Workspace", related_name="issue_properties", on_delete=models.CASCADE)
    issue_type = models.ForeignKey("db.IssueType", related_name="issue_properties", on_delete=models.CASCADE)
    # `name` is the stable api key, `display_name` is what the UI renders
    name = models.CharField(max_length=255)
    display_name = models.CharField(max_length=255)
    description = models.TextField(blank=True)
    property_type = models.CharField(max_length=255, choices=PropertyTypeEnum.choices)
    relation_type = models.CharField(max_length=255, choices=RelationTypeEnum.choices, null=True, blank=True)
    is_required = models.BooleanField(default=False)
    is_active = models.BooleanField(default=True)
    is_multi = models.BooleanField(default=False)
    default_value = models.JSONField(default=list)
    settings = models.JSONField(default=dict)
    sort_order = models.FloatField(default=65535)
    logo_props = models.JSONField(default=dict)
    external_source = models.CharField(max_length=255, null=True, blank=True)
    external_id = models.CharField(max_length=255, null=True, blank=True)

    class Meta:
        constraints = [
            models.UniqueConstraint(
                fields=["issue_type", "name"],
                condition=Q(deleted_at__isnull=True),
                name="issue_property_unique_issue_type_name_when_deleted_at_null",
            )
        ]
        verbose_name = "Issue Property"
        verbose_name_plural = "Issue Properties"
        db_table = "issue_properties"
        ordering = ("sort_order",)

    def __str__(self):
        return f"{self.issue_type} - {self.name}"


class IssuePropertyOption(BaseModel):
    """One selectable choice of an ``OPTION`` property."""

    workspace = models.ForeignKey("db.Workspace", related_name="issue_property_options", on_delete=models.CASCADE)
    property = models.ForeignKey("db.IssueProperty", related_name="options", on_delete=models.CASCADE)
    parent = models.ForeignKey("self", related_name="children", on_delete=models.CASCADE, null=True, blank=True)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True)
    is_active = models.BooleanField(default=True)
    is_default = models.BooleanField(default=False)
    sort_order = models.FloatField(default=65535)
    logo_props = models.JSONField(default=dict)
    external_source = models.CharField(max_length=255, null=True, blank=True)
    external_id = models.CharField(max_length=255, null=True, blank=True)

    class Meta:
        constraints = [
            models.UniqueConstraint(
                fields=["property", "name"],
                condition=Q(deleted_at__isnull=True),
                name="issue_property_option_unique_property_name_when_deleted_at_null",
            )
        ]
        verbose_name = "Issue Property Option"
        verbose_name_plural = "Issue Property Options"
        db_table = "issue_property_options"
        ordering = ("sort_order",)

    def __str__(self):
        return f"{self.property} - {self.name}"


class IssuePropertyValue(ProjectBaseModel):
    """One value of one property on one work item.

    Multi valued properties are several rows. The value lives in a typed column
    rather than in a JSON blob so that filtering, ordering and grouping in later
    phases can use an index.

    A row hangs off either a work item or a workspace draft — the create modal can
    fill in properties before the work item exists, and a draft lives in its own
    table. Converting the draft moves the values onto the work item it creates.
    """

    issue = models.ForeignKey(
        "db.Issue", related_name="property_values", on_delete=models.CASCADE, null=True, blank=True
    )
    draft_issue = models.ForeignKey(
        "db.DraftIssue", related_name="property_values", on_delete=models.CASCADE, null=True, blank=True
    )
    property = models.ForeignKey("db.IssueProperty", related_name="values", on_delete=models.CASCADE)
    value_text = models.TextField(null=True, blank=True)
    value_decimal = models.FloatField(null=True, blank=True)
    value_boolean = models.BooleanField(null=True, blank=True)
    value_datetime = models.DateTimeField(null=True, blank=True)
    # option id for OPTION, user or work item id for RELATION
    value_uuid = models.UUIDField(null=True, blank=True)
    external_source = models.CharField(max_length=255, null=True, blank=True)
    external_id = models.CharField(max_length=255, null=True, blank=True)

    class Meta:
        indexes = [
            models.Index(fields=["issue", "property"], name="ipv_issue_property_idx"),
            models.Index(fields=["draft_issue", "property"], name="ipv_draft_property_idx"),
            models.Index(fields=["property", "value_uuid"], name="ipv_property_uuid_idx"),
            models.Index(fields=["property", "value_decimal"], name="ipv_property_decimal_idx"),
            models.Index(fields=["property", "value_datetime"], name="ipv_property_datetime_idx"),
            models.Index(fields=["property", "value_boolean"], name="ipv_property_boolean_idx"),
        ]
        verbose_name = "Issue Property Value"
        verbose_name_plural = "Issue Property Values"
        db_table = "issue_property_values"
        ordering = ("created_at",)

    def __str__(self):
        return f"{self.issue or self.draft_issue} - {self.property}"


class IssuePropertyActivity(ProjectBaseModel):
    """Audit trail of property value changes, rendered by the detail activity feed."""

    issue = models.ForeignKey("db.Issue", related_name="property_activities", on_delete=models.CASCADE)
    property = models.ForeignKey("db.IssueProperty", related_name="activities", on_delete=models.SET_NULL, null=True)
    action = models.CharField(max_length=255, choices=IssuePropertyActionEnum.choices)
    old_value = models.TextField(null=True, blank=True)
    new_value = models.TextField(null=True, blank=True)
    old_identifier = models.UUIDField(null=True)
    new_identifier = models.UUIDField(null=True)
    comment = models.TextField(blank=True)
    actor = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        on_delete=models.SET_NULL,
        null=True,
        related_name="issue_property_activities",
    )
    epoch = models.FloatField(null=True)

    class Meta:
        indexes = [models.Index(fields=["issue", "property"], name="ipa_issue_property_idx")]
        verbose_name = "Issue Property Activity"
        verbose_name_plural = "Issue Property Activities"
        db_table = "issue_property_activities"
        ordering = ("-created_at",)

    def __str__(self):
        return f"{self.issue} - {self.property}"

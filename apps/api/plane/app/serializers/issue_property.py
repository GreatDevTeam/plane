# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Third party imports
from rest_framework import serializers

# Module imports
from .base import BaseSerializer
from plane.db.models import IssueProperty, IssuePropertyActivity, IssuePropertyOption, PropertyTypeEnum


class IssuePropertySerializer(BaseSerializer):
    class Meta:
        model = IssueProperty
        fields = [
            "id",
            "name",
            "display_name",
            "description",
            "property_type",
            "relation_type",
            "is_required",
            "is_active",
            "is_multi",
            "default_value",
            "settings",
            "sort_order",
            "logo_props",
            "issue_type_id",
            "workspace_id",
            "external_source",
            "external_id",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]
        read_only_fields = [
            "id",
            "issue_type_id",
            "workspace_id",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]

    def validate_name(self, value):
        name = (value or "").strip()
        if not name:
            raise serializers.ValidationError("Name cannot be empty")

        issue_type_id = self.context.get("issue_type_id") or getattr(self.instance, "issue_type_id", None)
        duplicate = IssueProperty.objects.filter(issue_type_id=issue_type_id, name__iexact=name)
        if self.instance:
            duplicate = duplicate.exclude(pk=self.instance.pk)
        if duplicate.exists():
            raise serializers.ValidationError("A property with this name already exists on this work item type")

        return name

    def validate(self, attrs):
        # `property_type` is immutable — the values already stored sit in the column
        # that matches the original type and would silently read back as empty.
        if self.instance and "property_type" in attrs and attrs["property_type"] != self.instance.property_type:
            raise serializers.ValidationError({"property_type": "The type of a property cannot be changed"})

        property_type = attrs.get("property_type") or getattr(self.instance, "property_type", None)
        relation_type = attrs.get("relation_type", getattr(self.instance, "relation_type", None))

        if property_type == PropertyTypeEnum.RELATION and not relation_type:
            raise serializers.ValidationError(
                {"relation_type": "A relation property has to declare what it relates to"}
            )
        if property_type != PropertyTypeEnum.RELATION and relation_type:
            raise serializers.ValidationError(
                {"relation_type": "Only a relation property can declare what it relates to"}
            )

        return attrs


class IssuePropertyOptionSerializer(BaseSerializer):
    # `parent_id` would otherwise resolve to the model attribute and come out read only
    parent_id = serializers.PrimaryKeyRelatedField(
        source="parent",
        queryset=IssuePropertyOption.objects.all(),
        required=False,
        allow_null=True,
    )

    class Meta:
        model = IssuePropertyOption
        fields = [
            "id",
            "name",
            "description",
            "is_active",
            "is_default",
            "sort_order",
            "logo_props",
            "parent_id",
            "property_id",
            "workspace_id",
            "external_source",
            "external_id",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]
        read_only_fields = [
            "id",
            "property_id",  # set from the url, not from the body
            "workspace_id",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]

    def validate_name(self, value):
        name = (value or "").strip()
        if not name:
            raise serializers.ValidationError("Name cannot be empty")

        property_id = self.context.get("property_id") or getattr(self.instance, "property_id", None)
        duplicate = IssuePropertyOption.objects.filter(property_id=property_id, name__iexact=name)
        if self.instance:
            duplicate = duplicate.exclude(pk=self.instance.pk)
        if duplicate.exists():
            raise serializers.ValidationError("An option with this name already exists on this property")

        return name

    def validate_parent_id(self, value):
        if value is None:
            return value

        property_id = self.context.get("property_id") or getattr(self.instance, "property_id", None)
        if str(value.property_id) != str(property_id):
            raise serializers.ValidationError("The parent has to be an option of the same property")
        if self.instance and value.pk == self.instance.pk:
            raise serializers.ValidationError("An option cannot be its own parent")

        return value


class IssuePropertyActivitySerializer(BaseSerializer):
    class Meta:
        model = IssuePropertyActivity
        fields = [
            "id",
            "issue_id",
            "property_id",
            "action",
            "old_value",
            "new_value",
            "old_identifier",
            "new_identifier",
            "comment",
            "actor",
            "epoch",
            "project_id",
            "workspace_id",
            "created_at",
        ]
        read_only_fields = fields

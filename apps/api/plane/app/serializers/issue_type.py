# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Third party imports
from rest_framework import serializers

# Module imports
from .base import BaseSerializer
from plane.db.models import IssueType, ProjectIssueType


class IssueTypeSerializer(BaseSerializer):
    project_ids = serializers.SerializerMethodField()

    class Meta:
        model = IssueType
        fields = [
            "id",
            "name",
            "description",
            "logo_props",
            "is_epic",
            "is_default",
            "is_active",
            "level",
            "workspace_id",
            "project_ids",
            "external_source",
            "external_id",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]
        read_only_fields = [
            "id",
            "workspace_id",
            "project_ids",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
        ]

    def get_project_ids(self, obj) -> list[str]:
        # `project_issue_types` is prefetched by the viewsets, so this stays a single query
        return [str(project_issue_type.project_id) for project_issue_type in obj.project_issue_types.all()]

    def validate_name(self, value):
        name = (value or "").strip()
        if not name:
            raise serializers.ValidationError("Name cannot be empty")

        workspace_id = self.context.get("workspace_id") or getattr(self.instance, "workspace_id", None)
        duplicate = IssueType.objects.filter(workspace_id=workspace_id, name__iexact=name)
        if self.instance:
            duplicate = duplicate.exclude(pk=self.instance.pk)
        if duplicate.exists():
            raise serializers.ValidationError("A work item type with this name already exists in the workspace")

        return name


class ProjectIssueTypeSerializer(BaseSerializer):
    class Meta:
        model = ProjectIssueType
        fields = ["id", "project_id", "workspace_id", "issue_type_id", "level", "is_default"]
        read_only_fields = ["id", "project_id", "workspace_id", "issue_type_id"]

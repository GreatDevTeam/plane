# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Third party imports
from rest_framework import serializers

# Module imports
from .base import BaseSerializer
from plane.db.models import Page, PageLabel, Label, ProjectPage, Project


class PageSerializer(BaseSerializer):
    label_ids = serializers.ListField(child=serializers.UUIDField(), read_only=True)
    project_ids = serializers.ListField(child=serializers.UUIDField(), read_only=True)

    class Meta:
        model = Page
        fields = [
            "id",
            "name",
            "owned_by",
            "access",
            "color",
            "parent",
            "is_locked",
            "archived_at",
            "workspace",
            "created_at",
            "updated_at",
            "created_by",
            "updated_by",
            "view_props",
            "logo_props",
            "label_ids",
            "project_ids",
            "external_id",
            "external_source",
        ]
        read_only_fields = ["workspace", "owned_by"]


class PageDetailSerializer(PageSerializer):
    description_html = serializers.CharField(read_only=True)

    class Meta(PageSerializer.Meta):
        fields = PageSerializer.Meta.fields + ["description_html"]


class PageCreateUpdateSerializer(BaseSerializer):
    labels = serializers.ListField(
        child=serializers.PrimaryKeyRelatedField(queryset=Label.objects.all()),
        write_only=True,
        required=False,
    )

    class Meta:
        model = Page
        fields = [
            "name",
            "description_html",
            "access",
            "color",
            "parent",
            "view_props",
            "logo_props",
            "labels",
            "external_id",
            "external_source",
        ]

    def create(self, validated_data):
        labels = validated_data.pop("labels", None)
        project_id = self.context["project_id"]
        owned_by_id = self.context["owned_by_id"]

        project = Project.objects.get(pk=project_id)

        page = Page.objects.create(
            **validated_data,
            owned_by_id=owned_by_id,
            workspace_id=project.workspace_id,
        )

        ProjectPage.objects.create(
            workspace_id=page.workspace_id,
            project_id=project_id,
            page_id=page.id,
            created_by_id=page.created_by_id,
            updated_by_id=page.updated_by_id,
        )

        if labels is not None:
            PageLabel.objects.bulk_create(
                [
                    PageLabel(
                        label=label,
                        page=page,
                        workspace_id=page.workspace_id,
                        created_by_id=page.created_by_id,
                        updated_by_id=page.updated_by_id,
                    )
                    for label in labels
                ],
                batch_size=10,
            )

        return page

    def update(self, instance, validated_data):
        labels = validated_data.pop("labels", None)
        if labels is not None:
            PageLabel.objects.filter(page=instance).delete()
            PageLabel.objects.bulk_create(
                [
                    PageLabel(
                        label=label,
                        page=instance,
                        workspace_id=instance.workspace_id,
                        created_by_id=instance.created_by_id,
                        updated_by_id=instance.updated_by_id,
                    )
                    for label in labels
                ],
                batch_size=10,
            )

        return super().update(instance, validated_data)

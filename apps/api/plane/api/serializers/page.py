# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Third party imports
from rest_framework import serializers

# Module imports
from .base import BaseSerializer
from plane.db.models import Page, PageLabel, Label, ProjectPage, Project
from plane.utils.live_server import replace_document_content


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
    parent_id = serializers.UUIDField(write_only=True, required=False, allow_null=True)

    class Meta:
        model = Page
        fields = [
            "name",
            "description_html",
            "access",
            "color",
            "parent",
            "parent_id",
            "view_props",
            "logo_props",
            "labels",
            "external_id",
            "external_source",
        ]

    def validate(self, data):
        # Allow parent_id as an alias for parent
        if "parent_id" in data:
            parent_id = data.pop("parent_id")
            if parent_id is not None:
                try:
                    data["parent"] = Page.objects.get(pk=parent_id)
                except Page.DoesNotExist:
                    raise serializers.ValidationError({"parent_id": "Page not found."})
            else:
                data["parent"] = None
        return data

    def validate_parent(self, value):
        if value is None:
            return value
        project_id = self.context.get("project_id")
        if project_id:
            if not ProjectPage.objects.filter(
                page=value,
                project_id=project_id,
                deleted_at__isnull=True,
            ).exists():
                raise serializers.ValidationError("Parent page must belong to the same project.")
        page_id = self.context.get("page_id")
        if page_id and str(value.pk) == str(page_id):
            raise serializers.ValidationError("A page cannot be its own parent.")
        return value

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
        # The editor renders a page from its collaborative (Yjs) snapshot stored in
        # description_binary, not from description_html, and the page title lives in
        # that snapshot as well. Writing the HTML or the name without the snapshot
        # would make the change invisible in the UI, so the live server rewrites the
        # content inside the existing snapshot and hands the update to any editor that
        # has the page open. It runs first so that a live server that cannot be reached
        # leaves the page untouched instead of half updated.
        self._sync_collaborative_snapshot(instance, validated_data)

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

    def _sync_collaborative_snapshot(self, instance, validated_data):
        changed = {
            field: validated_data[field]
            for field in ("description_html", "name")
            if field in validated_data and validated_data[field] != getattr(instance, field)
        }
        if not changed:
            return

        if not instance.description_binary:
            # The page has never been opened in the editor, so there is no snapshot the clients
            # could hold: the live server builds one from description_html and the page name the
            # first time the page is opened.
            return

        document = replace_document_content(
            document_id=instance.id,
            description_html=changed.get("description_html"),
            name=changed.get("name"),
            description_binary=instance.description_binary,
        )

        instance.description_binary = document["description_binary"]
        instance.description_json = document["description_json"]
        # keep the HTML in sync with the snapshot it was generated from
        validated_data["description_html"] = document["description_html"]

# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Django imports
from django.db import transaction

# Third party imports
from rest_framework import status
from rest_framework.response import Response

# Module imports
from .. import BaseViewSet
from plane.app.permissions import ROLE, allow_permission
from plane.app.serializers import IssueTypeSerializer, ProjectIssueTypeSerializer
from plane.db.models import IssueType, Project, ProjectIssueType, Workspace


class IssueTypeViewSet(BaseViewSet):
    """Workspace level CRUD for work item types."""

    model = IssueType
    serializer_class = IssueTypeSerializer

    def get_queryset(self):
        return (
            IssueType.objects.filter(workspace__slug=self.kwargs.get("slug"))
            .prefetch_related("project_issue_types")
            .order_by("level", "name")
        )

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def list(self, request, slug):
        serializer = IssueTypeSerializer(self.get_queryset(), many=True)
        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def retrieve(self, request, slug, pk):
        issue_type = self.get_queryset().filter(pk=pk).first()
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)
        return Response(IssueTypeSerializer(issue_type).data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def create(self, request, slug):
        workspace = Workspace.objects.get(slug=slug)
        serializer = IssueTypeSerializer(data=request.data, context={"workspace_id": workspace.id})
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        with transaction.atomic():
            if serializer.validated_data.get("is_default"):
                IssueType.objects.filter(workspace_id=workspace.id, is_default=True).update(is_default=False)
            serializer.save(workspace_id=workspace.id)

        return Response(serializer.data, status=status.HTTP_201_CREATED)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def partial_update(self, request, slug, pk):
        issue_type = self.get_queryset().filter(pk=pk).first()
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)

        serializer = IssueTypeSerializer(
            issue_type, data=request.data, partial=True, context={"workspace_id": issue_type.workspace_id}
        )
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        with transaction.atomic():
            if serializer.validated_data.get("is_default"):
                IssueType.objects.filter(workspace_id=issue_type.workspace_id, is_default=True).exclude(pk=pk).update(
                    is_default=False
                )
            serializer.save()

        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def destroy(self, request, slug, pk):
        issue_type = self.get_queryset().filter(pk=pk).first()
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)

        if issue_type.is_default:
            return Response(
                {"error": "The default work item type cannot be deleted"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        issue_type.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)


class ProjectIssueTypeViewSet(BaseViewSet):
    """Per project enablement of work item types."""

    model = IssueType
    serializer_class = IssueTypeSerializer

    def get_queryset(self):
        return (
            IssueType.objects.filter(
                workspace__slug=self.kwargs.get("slug"),
                project_issue_types__project_id=self.kwargs.get("project_id"),
                project_issue_types__deleted_at__isnull=True,
            )
            .prefetch_related("project_issue_types")
            .order_by("level", "name")
            .distinct()
        )

    def get_project_issue_type(self, project_id, issue_type_id):
        return ProjectIssueType.objects.filter(project_id=project_id, issue_type_id=issue_type_id).first()

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST])
    def list(self, request, slug, project_id):
        serializer = IssueTypeSerializer(self.get_queryset(), many=True)
        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST])
    def retrieve(self, request, slug, project_id, pk):
        issue_type = self.get_queryset().filter(pk=pk).first()
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)
        return Response(IssueTypeSerializer(issue_type).data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN])
    def create(self, request, slug, project_id):
        """Enable an existing work item type on the project, or create one and enable it."""
        project = Project.objects.get(pk=project_id, workspace__slug=slug)
        issue_type_id = request.data.get("issue_type_id")

        if issue_type_id:
            issue_type = IssueType.objects.filter(workspace_id=project.workspace_id, pk=issue_type_id).first()
            if issue_type is None:
                return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)
            if self.get_project_issue_type(project_id, issue_type.id):
                return Response(
                    {"error": "The work item type is already enabled on this project"},
                    status=status.HTTP_400_BAD_REQUEST,
                )
        else:
            serializer = IssueTypeSerializer(data=request.data, context={"workspace_id": project.workspace_id})
            if not serializer.is_valid():
                return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)
            issue_type = serializer.save(workspace_id=project.workspace_id, is_default=False)

        with transaction.atomic():
            is_default = bool(request.data.get("is_default", False))
            if is_default:
                ProjectIssueType.objects.filter(project_id=project_id, is_default=True).update(is_default=False)
            project_issue_type = ProjectIssueType(
                project=project,
                issue_type=issue_type,
                level=request.data.get("level", 0),
                is_default=is_default,
            )
            project_issue_type.save()

        issue_type.refresh_from_db()
        return Response(IssueTypeSerializer(issue_type).data, status=status.HTTP_201_CREATED)

    @allow_permission([ROLE.ADMIN])
    def partial_update(self, request, slug, project_id, pk):
        """Update a work item type and/or its project level enablement settings."""
        issue_type = self.get_queryset().filter(pk=pk).first()
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)

        project_issue_type = self.get_project_issue_type(project_id, issue_type.id)

        serializer = IssueTypeSerializer(
            issue_type, data=request.data, partial=True, context={"workspace_id": issue_type.workspace_id}
        )
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        project_serializer = ProjectIssueTypeSerializer(project_issue_type, data=request.data, partial=True)
        if not project_serializer.is_valid():
            return Response(project_serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        with transaction.atomic():
            serializer.save()
            if project_serializer.validated_data.get("is_default"):
                ProjectIssueType.objects.filter(project_id=project_id, is_default=True).exclude(
                    pk=project_issue_type.pk
                ).update(is_default=False)
            project_serializer.save()

        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN])
    def destroy(self, request, slug, project_id, pk):
        """Disable the work item type on this project — the type itself is kept."""
        project_issue_type = self.get_project_issue_type(project_id, pk)
        if project_issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)

        if project_issue_type.is_default:
            return Response(
                {"error": "The default work item type cannot be disabled"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        project_issue_type.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)

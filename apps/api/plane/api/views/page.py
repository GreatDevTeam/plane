# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Django imports
from datetime import datetime

from django.db import connection
from django.db.models import (
    Q,
    Value,
    UUIDField,
)
from django.contrib.postgres.aggregates import ArrayAgg
from django.contrib.postgres.fields import ArrayField
from django.db.models.functions import Coalesce

# Third party imports
from rest_framework import status
from rest_framework.response import Response
from drf_spectacular.utils import OpenApiRequest, OpenApiResponse

# Module imports
from plane.api.serializers import (
    PageSerializer,
    PageDetailSerializer,
    PageCreateUpdateSerializer,
)
from plane.app.permissions import ProjectPagePermission
from plane.db.models import Page, Project, ProjectMember
from plane.utils.openapi.decorators import page_docs
from plane.utils.openapi import (
    CURSOR_PARAMETER,
    PER_PAGE_PARAMETER,
    PAGE_ID_PARAMETER,
    DELETED_RESPONSE,
    ARCHIVED_RESPONSE,
    UNARCHIVED_RESPONSE,
    SEARCH_PARAMETER,
    PAGE_NOT_FOUND_RESPONSE,
    PAGE_EXAMPLE,
    PAGE_CREATE_EXAMPLE,
    PAGE_UPDATE_EXAMPLE,
    create_paginated_response,
)

from .base import BaseAPIView


def _archive_page_and_descendants(page_id, archived_at):
    sql = """
    WITH RECURSIVE descendants AS (
        SELECT id FROM pages WHERE id = %s
        UNION ALL
        SELECT pages.id FROM pages, descendants WHERE pages.parent_id = descendants.id
    )
    UPDATE pages SET archived_at = %s WHERE id IN (SELECT id FROM descendants);
    """
    with connection.cursor() as cursor:
        cursor.execute(sql, [page_id, archived_at])


class PageListCreateAPIEndpoint(BaseAPIView):
    """Page list and create endpoint"""

    permission_classes = [ProjectPagePermission]
    use_read_replica = True

    def get_queryset(self):
        return (
            Page.objects.filter(workspace__slug=self.kwargs.get("slug"))
            .filter(
                projects__id=self.kwargs.get("project_id"),
                project_pages__deleted_at__isnull=True,
            )
            .filter(
                projects__project_projectmember__member=self.request.user,
                projects__project_projectmember__is_active=True,
                projects__archived_at__isnull=True,
            )
            .filter(parent__isnull=True)
            .filter(Q(owned_by=self.request.user) | Q(access=Page.PUBLIC_ACCESS))
            .select_related("workspace", "owned_by")
            .annotate(
                label_ids=Coalesce(
                    ArrayAgg(
                        "page_labels__label_id",
                        distinct=True,
                        filter=~Q(page_labels__label_id__isnull=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
                project_ids=Coalesce(
                    ArrayAgg(
                        "projects__id",
                        distinct=True,
                        filter=~Q(projects__id=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
            )
            .distinct()
        )

    @page_docs(
        operation_id="list_pages",
        summary="List pages",
        description="List all pages in a project",
        parameters=[CURSOR_PARAMETER, PER_PAGE_PARAMETER],
        responses={
            200: create_paginated_response(PageSerializer, "Page", "List of pages", example_name="List of pages"),
        },
    )
    def get(self, request, slug, project_id):
        queryset = self.get_queryset()
        project = Project.objects.get(pk=project_id)

        if (
            ProjectMember.objects.filter(
                workspace__slug=slug,
                project_id=project_id,
                member=request.user,
                role=5,
                is_active=True,
            ).exists()
            and not project.guest_view_all_features
        ):
            queryset = queryset.filter(owned_by=request.user)

        return self.paginate(
            request=request,
            queryset=queryset,
            on_results=lambda pages: PageSerializer(pages, many=True).data,
            default_per_page=20,
        )

    @page_docs(
        operation_id="create_page",
        summary="Create a page",
        description="Create a new page in a project",
        request=OpenApiRequest(request=PageCreateUpdateSerializer),
        responses={
            201: OpenApiResponse(description="Page created", response=PageDetailSerializer, examples=[PAGE_EXAMPLE]),
        },
    )
    def post(self, request, slug, project_id):
        serializer = PageCreateUpdateSerializer(
            data=request.data,
            context={
                "project_id": project_id,
                "owned_by_id": request.user.id,
            },
        )

        if serializer.is_valid():
            page = serializer.save()
            page = (
                Page.objects.filter(pk=page.id)
                .annotate(
                    label_ids=Coalesce(
                        ArrayAgg(
                            "page_labels__label_id",
                            distinct=True,
                            filter=~Q(page_labels__label_id__isnull=True),
                        ),
                        Value([], output_field=ArrayField(UUIDField())),
                    ),
                    project_ids=Coalesce(
                        ArrayAgg(
                            "projects__id",
                            distinct=True,
                            filter=~Q(projects__id=True),
                        ),
                        Value([], output_field=ArrayField(UUIDField())),
                    ),
                )
                .first()
            )
            return Response(PageDetailSerializer(page).data, status=status.HTTP_201_CREATED)
        return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)


class PageDetailAPIEndpoint(BaseAPIView):
    """Page retrieve, update and delete endpoint"""

    permission_classes = [ProjectPagePermission]
    use_read_replica = True

    def get_queryset(self):
        return (
            Page.objects.filter(workspace__slug=self.kwargs.get("slug"))
            .filter(
                projects__id=self.kwargs.get("project_id"),
                project_pages__deleted_at__isnull=True,
            )
            .filter(Q(owned_by=self.request.user) | Q(access=Page.PUBLIC_ACCESS))
            .select_related("workspace", "owned_by")
            .annotate(
                label_ids=Coalesce(
                    ArrayAgg(
                        "page_labels__label_id",
                        distinct=True,
                        filter=~Q(page_labels__label_id__isnull=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
                project_ids=Coalesce(
                    ArrayAgg(
                        "projects__id",
                        distinct=True,
                        filter=~Q(projects__id=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
            )
            .distinct()
        )

    @page_docs(
        operation_id="retrieve_page",
        summary="Retrieve a page",
        description="Retrieve a page by its ID",
        parameters=[PAGE_ID_PARAMETER],
        responses={
            200: OpenApiResponse(description="Page", response=PageDetailSerializer, examples=[PAGE_EXAMPLE]),
            404: PAGE_NOT_FOUND_RESPONSE,
        },
    )
    def get(self, request, slug, project_id, pk):
        page = self.get_queryset().filter(pk=pk).first()
        if page is None:
            return Response({"error": "Page not found"}, status=status.HTTP_404_NOT_FOUND)
        return Response(PageDetailSerializer(page).data, status=status.HTTP_200_OK)

    @page_docs(
        operation_id="update_page",
        summary="Update a page",
        description="Update a page by its ID",
        parameters=[PAGE_ID_PARAMETER],
        request=OpenApiRequest(request=PageCreateUpdateSerializer),
        responses={
            200: OpenApiResponse(description="Page updated", response=PageDetailSerializer, examples=[PAGE_EXAMPLE]),
            404: PAGE_NOT_FOUND_RESPONSE,
        },
    )
    def patch(self, request, slug, project_id, pk):
        try:
            page = Page.objects.get(
                pk=pk,
                workspace__slug=slug,
                projects__id=project_id,
                project_pages__deleted_at__isnull=True,
            )
        except Page.DoesNotExist:
            return Response({"error": "Page not found"}, status=status.HTTP_404_NOT_FOUND)

        if page.is_locked:
            return Response({"error": "Page is locked"}, status=status.HTTP_400_BAD_REQUEST)

        if page.access != request.data.get("access", page.access) and page.owned_by_id != request.user.id:
            return Response(
                {"error": "Access cannot be updated since this page is owned by someone else"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        serializer = PageCreateUpdateSerializer(page, data=request.data, partial=True)
        if serializer.is_valid():
            serializer.save()
            page = self.get_queryset().filter(pk=pk).first()
            return Response(PageDetailSerializer(page).data, status=status.HTTP_200_OK)
        return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

    @page_docs(
        operation_id="delete_page",
        summary="Delete a page",
        description="Delete a page by its ID. The page must be archived before it can be deleted.",
        parameters=[PAGE_ID_PARAMETER],
        responses={
            204: DELETED_RESPONSE,
            400: OpenApiResponse(description="Page must be archived before deleting"),
            403: OpenApiResponse(description="Only admin or owner can delete the page"),
            404: PAGE_NOT_FOUND_RESPONSE,
        },
    )
    def delete(self, request, slug, project_id, pk):
        try:
            page = Page.objects.get(
                pk=pk,
                workspace__slug=slug,
                projects__id=project_id,
                project_pages__deleted_at__isnull=True,
            )
        except Page.DoesNotExist:
            return Response({"error": "Page not found"}, status=status.HTTP_404_NOT_FOUND)

        if page.archived_at is None:
            return Response(
                {"error": "The page should be archived before deleting"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        if page.owned_by_id != request.user.id and (
            not ProjectMember.objects.filter(
                workspace__slug=slug,
                member=request.user,
                role=20,
                project_id=project_id,
                is_active=True,
            ).exists()
        ):
            return Response(
                {"error": "Only admin or owner can delete the page"},
                status=status.HTTP_403_FORBIDDEN,
            )

        Page.objects.filter(
            parent_id=pk,
            projects__id=project_id,
            workspace__slug=slug,
            project_pages__deleted_at__isnull=True,
        ).update(parent=None)

        page.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)


class PageArchiveUnarchiveAPIEndpoint(BaseAPIView):
    """Page archive and unarchive endpoint"""

    permission_classes = [ProjectPagePermission]

    @page_docs(
        operation_id="archive_page",
        summary="Archive a page",
        description="Archive a page and all its descendants. Only the owner or an admin can archive a page.",
        parameters=[PAGE_ID_PARAMETER],
        responses={
            200: ARCHIVED_RESPONSE,
            400: OpenApiResponse(description="Only the owner or admin can archive the page"),
            404: PAGE_NOT_FOUND_RESPONSE,
        },
    )
    def post(self, request, slug, project_id, pk):
        try:
            page = Page.objects.get(
                pk=pk,
                workspace__slug=slug,
                projects__id=project_id,
                project_pages__deleted_at__isnull=True,
            )
        except Page.DoesNotExist:
            return Response({"error": "Page not found"}, status=status.HTTP_404_NOT_FOUND)

        if (
            ProjectMember.objects.filter(
                project_id=project_id, member=request.user, is_active=True, role__lte=15
            ).exists()
            and request.user.id != page.owned_by_id
        ):
            return Response(
                {"error": "Only the owner or admin can archive the page"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        archived_at = datetime.now()
        _archive_page_and_descendants(pk, archived_at)
        return Response({"archived_at": str(archived_at)}, status=status.HTTP_200_OK)

    @page_docs(
        operation_id="unarchive_page",
        summary="Unarchive a page",
        description="Unarchive a page and all its descendants. Only the owner or an admin can unarchive a page.",
        parameters=[PAGE_ID_PARAMETER],
        responses={
            204: UNARCHIVED_RESPONSE,
            400: OpenApiResponse(description="Only the owner or admin can unarchive the page"),
            404: PAGE_NOT_FOUND_RESPONSE,
        },
    )
    def delete(self, request, slug, project_id, pk):
        try:
            page = Page.objects.get(
                pk=pk,
                workspace__slug=slug,
                projects__id=project_id,
                project_pages__deleted_at__isnull=True,
            )
        except Page.DoesNotExist:
            return Response({"error": "Page not found"}, status=status.HTTP_404_NOT_FOUND)

        if (
            ProjectMember.objects.filter(
                project_id=project_id, member=request.user, is_active=True, role__lte=15
            ).exists()
            and request.user.id != page.owned_by_id
        ):
            return Response(
                {"error": "Only the owner or admin can unarchive the page"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        if page.parent_id and page.parent.archived_at:
            page.parent = None
            page.save(update_fields=["parent"])

        _archive_page_and_descendants(pk, None)
        return Response(status=status.HTTP_204_NO_CONTENT)


class PageSearchAPIEndpoint(BaseAPIView):
    """Search pages by name"""

    permission_classes = [ProjectPagePermission]
    use_read_replica = True

    @page_docs(
        operation_id="search_pages",
        summary="Search pages",
        description="Search for pages in a project by name.",
        parameters=[SEARCH_PARAMETER],
        responses={
            200: OpenApiResponse(
                description="List of matching pages",
                response=PageSerializer(many=True),
            ),
        },
    )
    def get(self, request, slug, project_id):
        query = request.query_params.get("search", "")

        pages = (
            Page.objects.filter(workspace__slug=slug)
            .filter(
                projects__id=project_id,
                project_pages__deleted_at__isnull=True,
            )
            .filter(
                projects__project_projectmember__member=request.user,
                projects__project_projectmember__is_active=True,
                projects__archived_at__isnull=True,
            )
            .filter(parent__isnull=True)
            .filter(Q(owned_by=request.user) | Q(access=Page.PUBLIC_ACCESS))
            .annotate(
                label_ids=Coalesce(
                    ArrayAgg(
                        "page_labels__label_id",
                        distinct=True,
                        filter=~Q(page_labels__label_id__isnull=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
                project_ids=Coalesce(
                    ArrayAgg(
                        "projects__id",
                        distinct=True,
                        filter=~Q(projects__id=True),
                    ),
                    Value([], output_field=ArrayField(UUIDField())),
                ),
            )
            .distinct()
        )

        if query:
            pages = pages.filter(name__icontains=query)

        return Response(PageSerializer(pages, many=True).data, status=status.HTTP_200_OK)

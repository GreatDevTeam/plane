# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Python imports
from collections import defaultdict

# Django imports
from django.db import transaction

# Third party imports
from rest_framework import status
from rest_framework.response import Response

# Module imports
from .. import BaseViewSet
from plane.app.permissions import ROLE, allow_permission
from plane.app.serializers import (
    IssuePropertyActivitySerializer,
    IssuePropertyOptionSerializer,
    IssuePropertySerializer,
)
from plane.db.models import (
    DraftIssue,
    Issue,
    IssueProperty,
    IssuePropertyActivity,
    IssuePropertyOption,
    IssuePropertyValue,
    IssueType,
)
from plane.utils.issue_property import properties_of, replace_property_values, values_map

# A page of the board is 100 work items; the cap keeps a hand written url from
# turning into an unbounded `IN (...)`
MAX_BULK_ISSUE_IDS = 500


class IssuePropertyViewSet(BaseViewSet):
    """CRUD for the properties of one work item type."""

    model = IssueProperty
    serializer_class = IssuePropertySerializer

    def get_queryset(self):
        return IssueProperty.objects.filter(
            workspace__slug=self.kwargs.get("slug"),
            issue_type_id=self.kwargs.get("issue_type_id"),
        ).order_by("sort_order", "created_at")

    def get_issue_type(self, slug, issue_type_id):
        return IssueType.objects.filter(workspace__slug=slug, pk=issue_type_id).first()

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def list(self, request, slug, issue_type_id):
        serializer = IssuePropertySerializer(self.get_queryset(), many=True)
        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def retrieve(self, request, slug, issue_type_id, pk):
        issue_property = self.get_queryset().filter(pk=pk).first()
        if issue_property is None:
            return Response({"error": "Property not found"}, status=status.HTTP_404_NOT_FOUND)
        return Response(IssuePropertySerializer(issue_property).data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def create(self, request, slug, issue_type_id):
        issue_type = self.get_issue_type(slug, issue_type_id)
        if issue_type is None:
            return Response({"error": "Work item type not found"}, status=status.HTTP_404_NOT_FOUND)

        serializer = IssuePropertySerializer(data=request.data, context={"issue_type_id": issue_type.id})
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        serializer.save(issue_type_id=issue_type.id, workspace_id=issue_type.workspace_id)
        return Response(serializer.data, status=status.HTTP_201_CREATED)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def partial_update(self, request, slug, issue_type_id, pk):
        issue_property = self.get_queryset().filter(pk=pk).first()
        if issue_property is None:
            return Response({"error": "Property not found"}, status=status.HTTP_404_NOT_FOUND)

        serializer = IssuePropertySerializer(
            issue_property,
            data=request.data,
            partial=True,
            context={"issue_type_id": issue_property.issue_type_id},
        )
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        serializer.save()
        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def destroy(self, request, slug, issue_type_id, pk):
        issue_property = self.get_queryset().filter(pk=pk).first()
        if issue_property is None:
            return Response({"error": "Property not found"}, status=status.HTTP_404_NOT_FOUND)

        issue_property.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)


class IssuePropertyOptionViewSet(BaseViewSet):
    """CRUD for the options of one ``OPTION`` property."""

    model = IssuePropertyOption
    serializer_class = IssuePropertyOptionSerializer

    def get_queryset(self):
        return IssuePropertyOption.objects.filter(
            workspace__slug=self.kwargs.get("slug"),
            property_id=self.kwargs.get("property_id"),
        ).order_by("sort_order", "created_at")

    def get_property(self, slug, property_id):
        return IssueProperty.objects.filter(workspace__slug=slug, pk=property_id).first()

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def list(self, request, slug, property_id):
        serializer = IssuePropertyOptionSerializer(self.get_queryset(), many=True)
        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def retrieve(self, request, slug, property_id, pk):
        option = self.get_queryset().filter(pk=pk).first()
        if option is None:
            return Response({"error": "Option not found"}, status=status.HTTP_404_NOT_FOUND)
        return Response(IssuePropertyOptionSerializer(option).data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def create(self, request, slug, property_id):
        issue_property = self.get_property(slug, property_id)
        if issue_property is None:
            return Response({"error": "Property not found"}, status=status.HTTP_404_NOT_FOUND)

        serializer = IssuePropertyOptionSerializer(data=request.data, context={"property_id": issue_property.id})
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        with transaction.atomic():
            if serializer.validated_data.get("is_default"):
                self.get_queryset().filter(is_default=True).update(is_default=False)
            serializer.save(property_id=issue_property.id, workspace_id=issue_property.workspace_id)

        return Response(serializer.data, status=status.HTTP_201_CREATED)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def partial_update(self, request, slug, property_id, pk):
        option = self.get_queryset().filter(pk=pk).first()
        if option is None:
            return Response({"error": "Option not found"}, status=status.HTTP_404_NOT_FOUND)

        serializer = IssuePropertyOptionSerializer(
            option, data=request.data, partial=True, context={"property_id": option.property_id}
        )
        if not serializer.is_valid():
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)

        with transaction.atomic():
            if serializer.validated_data.get("is_default"):
                self.get_queryset().filter(is_default=True).exclude(pk=pk).update(is_default=False)
            serializer.save()

        return Response(serializer.data, status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN], level="WORKSPACE")
    def destroy(self, request, slug, property_id, pk):
        option = self.get_queryset().filter(pk=pk).first()
        if option is None:
            return Response({"error": "Option not found"}, status=status.HTTP_404_NOT_FOUND)

        with transaction.atomic():
            # The values pointing at this option would otherwise read back as a
            # dangling id — drop them with it
            IssuePropertyValue.objects.filter(property_id=property_id, value_uuid=option.pk).delete()
            option.delete()

        return Response(status=status.HTTP_204_NO_CONTENT)


class IssuePropertyValueMixin:
    """Shared reading and writing of property values.

    The implementation lives in ``plane.utils.issue_property`` — the public API writes
    the same values through it, so it cannot hang off a view.
    """

    def properties_of(self, issue_type_ids):
        return properties_of(issue_type_ids)

    def values_map(self, owner_ids, properties, owner_field="issue"):
        return values_map(owner_ids, properties, owner_field)

    def replace_values(self, issue, property_values, actor, owner_field="issue"):
        return replace_property_values(issue, property_values, actor, owner_field)


class IssuePropertyValueViewSet(IssuePropertyValueMixin, BaseViewSet):
    """Read and replace the property values of one work item."""

    model = IssuePropertyValue

    def get_issue(self, slug, project_id, issue_id):
        return Issue.objects.filter(workspace__slug=slug, project_id=project_id, pk=issue_id).first()

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST])
    def list(self, request, slug, project_id, issue_id):
        issue = self.get_issue(slug, project_id, issue_id)
        if issue is None:
            return Response({"error": "Work item not found"}, status=status.HTTP_404_NOT_FOUND)

        properties = self.properties_of([issue.type_id])
        return Response(self.values_map([issue.id], properties)[str(issue.id)], status=status.HTTP_200_OK)

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER])
    def create(self, request, slug, project_id, issue_id):
        issue = self.get_issue(slug, project_id, issue_id)
        if issue is None:
            return Response({"error": "Work item not found"}, status=status.HTTP_404_NOT_FOUND)

        property_values = request.data.get("property_values", request.data)
        if not isinstance(property_values, dict):
            return Response(
                {"error": "property_values has to be a map of property id to values"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        values, errors = self.replace_values(issue, property_values, request.user)
        if errors:
            return Response(errors, status=status.HTTP_400_BAD_REQUEST)

        return Response(values, status=status.HTTP_200_OK)


class DraftIssuePropertyValueViewSet(IssuePropertyValueMixin, BaseViewSet):
    """Read and replace the property values of one workspace draft.

    The create modal fills in custom fields before the work item exists, so a draft
    saved out of it has to keep them; converting the draft carries them over
    (``WorkspaceDraftIssueViewSet.create_draft_to_issue``).
    """

    model = IssuePropertyValue

    def get_draft_issue(self, request, slug, draft_id):
        # drafts are private to whoever wrote them, as everywhere else
        return DraftIssue.objects.filter(workspace__slug=slug, pk=draft_id, created_by=request.user).first()

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def list(self, request, slug, draft_id):
        draft_issue = self.get_draft_issue(request, slug, draft_id)
        if draft_issue is None:
            return Response({"error": "Draft not found"}, status=status.HTTP_404_NOT_FOUND)

        properties = self.properties_of([draft_issue.type_id])
        return Response(
            self.values_map([draft_issue.id], properties, "draft_issue")[str(draft_issue.id)],
            status=status.HTTP_200_OK,
        )

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST], level="WORKSPACE")
    def create(self, request, slug, draft_id):
        draft_issue = self.get_draft_issue(request, slug, draft_id)
        if draft_issue is None:
            return Response({"error": "Draft not found"}, status=status.HTTP_404_NOT_FOUND)

        # values are validated against the project the draft is filed under, and
        # stored on a row that needs one
        if not draft_issue.project_id:
            return Response(
                {"error": "Project is required to set property values."},
                status=status.HTTP_400_BAD_REQUEST,
            )

        property_values = request.data.get("property_values", request.data)
        if not isinstance(property_values, dict):
            return Response(
                {"error": "property_values has to be a map of property id to values"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        values, errors = self.replace_values(draft_issue, property_values, request.user, "draft_issue")
        if errors:
            return Response(errors, status=status.HTTP_400_BAD_REQUEST)

        return Response(values, status=status.HTTP_200_OK)


class BulkIssuePropertyValueViewSet(IssuePropertyValueMixin, BaseViewSet):
    """Property values for a page of work items, in one request.

    The board and spreadsheet payloads are built with ``.values(...)`` and have to
    stay cheap, so the values of a page are fetched separately rather than joined
    into the work item query.
    """

    model = IssuePropertyValue

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST])
    def list(self, request, slug, project_id):
        raw_ids = request.GET.get("issue_ids", "")
        issue_ids = [issue_id for issue_id in raw_ids.split(",") if issue_id]

        if not issue_ids:
            return Response({"error": "issue_ids is required"}, status=status.HTTP_400_BAD_REQUEST)
        if len(issue_ids) > MAX_BULK_ISSUE_IDS:
            return Response(
                {"error": f"issue_ids cannot hold more than {MAX_BULK_ISSUE_IDS} work items"},
                status=status.HTTP_400_BAD_REQUEST,
            )

        issues = Issue.objects.filter(workspace__slug=slug, project_id=project_id, pk__in=issue_ids).values_list(
            "id", "type_id"
        )
        properties = self.properties_of({type_id for _, type_id in issues})

        # Each work item only carries the properties of its own type
        properties_by_type = defaultdict(list)
        for issue_property in properties:
            properties_by_type[issue_property.issue_type_id].append(issue_property)

        all_values = self.values_map([issue_id for issue_id, _ in issues], properties)
        return Response(
            {
                str(issue_id): {
                    str(issue_property.id): all_values[str(issue_id)][str(issue_property.id)]
                    for issue_property in properties_by_type[type_id]
                }
                for issue_id, type_id in issues
            },
            status=status.HTTP_200_OK,
        )


class IssuePropertyActivityViewSet(BaseViewSet):
    """The audit trail of one work item's property changes, for its activity feed.

    Kept off the work item activity endpoint: that one reads ``IssueActivity``, and a
    property change is a row of its own table, so the feed merges the two client side
    the way it already merges activities with comments.
    """

    model = IssuePropertyActivity
    serializer_class = IssuePropertyActivitySerializer

    @allow_permission([ROLE.ADMIN, ROLE.MEMBER, ROLE.GUEST])
    def list(self, request, slug, project_id, issue_id):
        # The feed polls with the timestamp of the last row it holds, so a refresh
        # asks only for what it has not seen
        filters = {}
        if request.GET.get("created_at__gt") is not None:
            filters["created_at__gt"] = request.GET.get("created_at__gt")

        activities = (
            IssuePropertyActivity.objects.filter(
                workspace__slug=slug,
                project_id=project_id,
                issue_id=issue_id,
                project__project_projectmember__member=request.user,
                project__project_projectmember__is_active=True,
                project__archived_at__isnull=True,
            )
            .filter(**filters)
            .order_by("created_at")
        )

        return Response(IssuePropertyActivitySerializer(activities, many=True).data, status=status.HTTP_200_OK)

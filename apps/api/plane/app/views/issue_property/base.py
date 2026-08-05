# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Python imports
from collections import defaultdict

# Django imports
from django.db import transaction
from django.utils import timezone

# Third party imports
from rest_framework import status
from rest_framework.response import Response

# Module imports
from .. import BaseViewSet
from plane.app.permissions import ROLE, allow_permission
from plane.app.serializers import IssuePropertyOptionSerializer, IssuePropertySerializer
from plane.db.models import (
    Issue,
    IssueProperty,
    IssuePropertyActionEnum,
    IssuePropertyActivity,
    IssuePropertyOption,
    IssuePropertyValue,
    IssueType,
)
from plane.utils.issue_property import PropertyValueError, coerce_values, serialize_value, value_field_for

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
    """Shared reading and writing of property values."""

    def properties_of(self, issue_type_ids):
        return IssueProperty.objects.filter(issue_type_id__in=issue_type_ids).order_by("sort_order", "created_at")

    def values_map(self, issue_ids, properties):
        """``{issue_id: {property_id: [value, …]}}`` for the given work items."""
        properties_by_id = {issue_property.id: issue_property for issue_property in properties}

        values = defaultdict(lambda: defaultdict(list))
        rows = IssuePropertyValue.objects.filter(
            issue_id__in=issue_ids, property_id__in=properties_by_id.keys()
        ).order_by("created_at")

        for row in rows:
            issue_property = properties_by_id[row.property_id]
            value = serialize_value(issue_property, row)
            if value is not None:
                values[str(row.issue_id)][str(row.property_id)].append(value)

        # A work item with no value for a property still has to carry the empty
        # list, otherwise the client cannot tell "not loaded" from "not set"
        return {
            str(issue_id): {
                str(issue_property.id): values[str(issue_id)].get(str(issue_property.id), [])
                for issue_property in properties
            }
            for issue_id in issue_ids
        }

    def replace_values(self, issue, property_values, actor):
        """Replace the values of the submitted properties on one work item.

        Properties that are not in the payload are left alone, so a partial save
        from the detail sidebar does not wipe the rest of the form.
        """
        properties = {str(issue_property.id): issue_property for issue_property in self.properties_of([issue.type_id])}

        coerced = {}
        errors = {}
        for property_id, raw_values in property_values.items():
            issue_property = properties.get(str(property_id))
            if issue_property is None:
                errors[str(property_id)] = "The property does not belong to the work item type"
                continue
            try:
                coerced[str(property_id)] = coerce_values(issue_property, raw_values, issue.project)
            except PropertyValueError as error:
                errors[error.property_id] = error.message

        if errors:
            return None, errors

        epoch = int(timezone.now().timestamp())
        with transaction.atomic():
            for property_id, values in coerced.items():
                issue_property = properties[property_id]
                existing = list(IssuePropertyValue.objects.filter(issue_id=issue.id, property_id=property_id))
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

                IssuePropertyValue.objects.filter(issue_id=issue.id, property_id=property_id).delete()
                IssuePropertyValue.objects.bulk_create(
                    [
                        IssuePropertyValue(
                            issue_id=issue.id,
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

                IssuePropertyActivity.objects.create(
                    issue_id=issue.id,
                    property_id=issue_property.id,
                    project_id=issue.project_id,
                    workspace_id=issue.workspace_id,
                    actor=actor,
                    action=self.action_for(old_values, new_values),
                    old_value=", ".join(str(value) for value in old_values) or None,
                    new_value=", ".join(str(value) for value in new_values) or None,
                    epoch=epoch,
                )

        return self.values_map([issue.id], properties.values())[str(issue.id)], None

    @staticmethod
    def action_for(old_values, new_values):
        if not old_values:
            return IssuePropertyActionEnum.CREATED
        if not new_values:
            return IssuePropertyActionEnum.DELETED
        return IssuePropertyActionEnum.UPDATED


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

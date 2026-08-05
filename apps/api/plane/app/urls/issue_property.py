# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

from django.urls import path

from plane.app.views import (
    BulkIssuePropertyValueViewSet,
    DraftIssuePropertyValueViewSet,
    IssuePropertyOptionViewSet,
    IssuePropertyValueViewSet,
    IssuePropertyViewSet,
)


urlpatterns = [
    path(
        "workspaces/<str:slug>/issue-types/<uuid:issue_type_id>/issue-properties/",
        IssuePropertyViewSet.as_view({"get": "list", "post": "create"}),
        name="issue-properties",
    ),
    path(
        "workspaces/<str:slug>/issue-types/<uuid:issue_type_id>/issue-properties/<uuid:pk>/",
        IssuePropertyViewSet.as_view({"get": "retrieve", "patch": "partial_update", "delete": "destroy"}),
        name="issue-property",
    ),
    path(
        "workspaces/<str:slug>/issue-properties/<uuid:property_id>/options/",
        IssuePropertyOptionViewSet.as_view({"get": "list", "post": "create"}),
        name="issue-property-options",
    ),
    path(
        "workspaces/<str:slug>/issue-properties/<uuid:property_id>/options/<uuid:pk>/",
        IssuePropertyOptionViewSet.as_view({"get": "retrieve", "patch": "partial_update", "delete": "destroy"}),
        name="issue-property-option",
    ),
    path(
        "workspaces/<str:slug>/projects/<uuid:project_id>/issues/<uuid:issue_id>/issue-property-values/",
        IssuePropertyValueViewSet.as_view({"get": "list", "post": "create"}),
        name="issue-property-values",
    ),
    path(
        "workspaces/<str:slug>/projects/<uuid:project_id>/issue-property-values/",
        BulkIssuePropertyValueViewSet.as_view({"get": "list"}),
        name="bulk-issue-property-values",
    ),
    path(
        "workspaces/<str:slug>/draft-issues/<uuid:draft_id>/issue-property-values/",
        DraftIssuePropertyValueViewSet.as_view({"get": "list", "post": "create"}),
        name="draft-issue-property-values",
    ),
]

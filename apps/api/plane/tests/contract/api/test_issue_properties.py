# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

"""Custom field values over the public API — what a webhook delivers and what an
import may hand back."""

import pytest
from rest_framework import status

from plane.db.models import Issue, IssueProperty, IssuePropertyOption, IssuePropertyValue, Project, ProjectMember
from plane.utils.issue_type import get_or_create_default_issue_type


@pytest.fixture
def project(db, workspace, create_user):
    project = Project.objects.create(name="Test Project", identifier="TP", workspace=workspace, created_by=create_user)
    ProjectMember.objects.create(project=project, member=create_user, role=20, is_active=True)
    return project


@pytest.fixture
def issue_type(project):
    return get_or_create_default_issue_type(project)


@pytest.fixture
def severity(issue_type):
    return IssueProperty.objects.create(
        workspace=issue_type.workspace,
        issue_type=issue_type,
        name="severity",
        display_name="Severity",
        property_type="TEXT",
    )


@pytest.fixture
def priority(issue_type):
    issue_property = IssueProperty.objects.create(
        workspace=issue_type.workspace,
        issue_type=issue_type,
        name="priority",
        display_name="Priority",
        property_type="OPTION",
    )
    IssuePropertyOption.objects.create(
        workspace=issue_type.workspace, property=issue_property, name="Blocker", sort_order=1
    )
    return issue_property


def issues_url(slug, project_id, pk=None):
    base_url = f"/api/v1/workspaces/{slug}/projects/{project_id}/issues/"
    return f"{base_url}{pk}/" if pk else base_url


@pytest.mark.contract
class TestWorkItemPropertyValues:
    @pytest.mark.django_db
    def test_create_accepts_values_keyed_by_property_id(self, api_key_client, workspace, project, severity):
        response = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {str(severity.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        issue = Issue.objects.get(pk=response.json()["id"])
        assert [value.value_text for value in IssuePropertyValue.objects.filter(issue=issue)] == ["High"]

    @pytest.mark.django_db
    def test_create_accepts_the_names_the_export_writes(self, api_key_client, workspace, project, priority):
        """The export renders a field by its name and an option by its own — an
        exported work item has to be importable without a lookup table."""
        response = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"priority": ["Blocker"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        value = IssuePropertyValue.objects.get(issue_id=response.json()["id"])
        assert str(value.value_uuid) == str(IssuePropertyOption.objects.get(property=priority).id)

    @pytest.mark.django_db
    def test_a_rejected_value_leaves_no_work_item_behind(self, api_key_client, workspace, project, priority):
        response = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"priority": ["Nonexistent"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert Issue.objects.filter(project=project, name="Work item").count() == 0

    @pytest.mark.django_db
    def test_a_value_of_another_type_is_refused(self, api_key_client, workspace, project, issue_type):
        other_type = get_or_create_default_issue_type(project)
        assert other_type == issue_type

        response = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"unknown": ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert "unknown" in response.json()

    @pytest.mark.django_db
    def test_update_replaces_the_values(self, api_key_client, workspace, project, severity):
        created = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"severity": ["High"]}},
            format="json",
        )

        response = api_key_client.patch(
            issues_url(workspace.slug, project.id, created.json()["id"]),
            {"property_values": {"severity": ["Low"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert [value.value_text for value in IssuePropertyValue.objects.all()] == ["Low"]

    @pytest.mark.django_db
    def test_the_payload_carries_the_values_back_by_api_name(self, api_key_client, workspace, project, severity):
        created = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"severity": ["High"]}},
            format="json",
        )

        response = api_key_client.get(issues_url(workspace.slug, project.id, created.json()["id"]))

        assert response.json()["property_values"] == {"severity": ["High"]}

    @pytest.mark.django_db
    def test_an_option_comes_back_as_its_id(self, api_key_client, workspace, project, priority):
        """A webhook consumer gets ids, not labels — what it receives is what the
        create and update endpoints take back."""
        created = api_key_client.post(
            issues_url(workspace.slug, project.id),
            {"name": "Work item", "property_values": {"priority": ["Blocker"]}},
            format="json",
        )

        response = api_key_client.get(issues_url(workspace.slug, project.id, created.json()["id"]))

        option = IssuePropertyOption.objects.get(property=priority)
        assert response.json()["property_values"] == {"priority": [str(option.id)]}

    @pytest.mark.django_db
    def test_a_work_item_without_values_carries_an_empty_map(self, api_key_client, workspace, project, severity):
        created = api_key_client.post(issues_url(workspace.slug, project.id), {"name": "Work item"}, format="json")

        response = api_key_client.get(issues_url(workspace.slug, project.id, created.json()["id"]))

        assert response.json()["property_values"] == {}

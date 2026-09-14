# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

import pytest
from rest_framework import status

from plane.db.models import Issue, IssueType, Project, ProjectIssueType, ProjectMember, State
from plane.utils.issue_type import get_default_issue_type, get_or_create_default_issue_type


@pytest.fixture
def project(db, workspace, create_user):
    project = Project.objects.create(name="Test Project", identifier="TP", workspace=workspace)
    ProjectMember.objects.create(project=project, member=create_user, role=20, is_active=True)
    return project


@pytest.fixture
def default_issue_type(project):
    return get_or_create_default_issue_type(project)


def workspace_url(slug, pk=None):
    base_url = f"/api/workspaces/{slug}/issue-types/"
    return f"{base_url}{pk}/" if pk else base_url


def project_url(slug, project_id, pk=None):
    base_url = f"/api/workspaces/{slug}/projects/{project_id}/issue-types/"
    return f"{base_url}{pk}/" if pk else base_url


@pytest.mark.contract
class TestDefaultIssueType:
    """The default type helper is what seeds new projects and backfills existing ones"""

    @pytest.mark.django_db
    def test_default_type_is_created_once_per_workspace(self, workspace, create_user):
        first = Project.objects.create(name="First", identifier="ONE", workspace=workspace)
        second = Project.objects.create(name="Second", identifier="TWO", workspace=workspace)

        first_type = get_or_create_default_issue_type(first)
        second_type = get_or_create_default_issue_type(second)

        assert first_type.id == second_type.id
        assert IssueType.objects.filter(workspace=workspace).count() == 1
        assert ProjectIssueType.objects.filter(issue_type=first_type).count() == 2

    @pytest.mark.django_db
    def test_get_or_create_is_idempotent(self, project):
        get_or_create_default_issue_type(project)
        get_or_create_default_issue_type(project)

        assert IssueType.objects.filter(workspace=project.workspace).count() == 1
        assert ProjectIssueType.objects.filter(project=project).count() == 1

    @pytest.mark.django_db
    def test_project_level_default_wins_over_workspace_default(self, project, default_issue_type):
        other = IssueType.objects.create(workspace=project.workspace, name="Bug", is_default=False)
        ProjectIssueType.objects.create(project=project, workspace=project.workspace, issue_type=other, is_default=True)
        ProjectIssueType.objects.filter(project=project, issue_type=default_issue_type).update(is_default=False)

        assert get_default_issue_type(project.id).id == other.id

    @pytest.mark.django_db
    def test_creating_a_project_seeds_the_default_type(self, session_client, workspace):
        response = session_client.post(
            f"/api/workspaces/{workspace.slug}/projects/",
            {"name": "Seeded Project", "identifier": "SEED"},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        project_id = response.data["id"]
        assert get_default_issue_type(project_id) is not None


@pytest.mark.contract
class TestWorkspaceIssueTypeEndpoint:
    @pytest.mark.django_db
    def test_list_returns_workspace_types(self, session_client, workspace, default_issue_type, project):
        response = session_client.get(workspace_url(workspace.slug))

        assert response.status_code == status.HTTP_200_OK
        assert len(response.data) == 1
        assert str(response.data[0]["id"]) == str(default_issue_type.id)
        assert response.data[0]["project_ids"] == [str(project.id)]

    @pytest.mark.django_db
    def test_create_issue_type(self, session_client, workspace):
        response = session_client.post(workspace_url(workspace.slug), {"name": "Bug"}, format="json")

        assert response.status_code == status.HTTP_201_CREATED
        assert IssueType.objects.filter(workspace=workspace, name="Bug").exists()

    @pytest.mark.django_db
    def test_create_rejects_duplicate_name(self, session_client, workspace, default_issue_type):
        response = session_client.post(
            workspace_url(workspace.slug), {"name": default_issue_type.name.lower()}, format="json"
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert IssueType.objects.filter(workspace=workspace).count() == 1

    @pytest.mark.django_db
    def test_create_rejects_empty_name(self, session_client, workspace):
        response = session_client.post(workspace_url(workspace.slug), {"name": "  "}, format="json")

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_marking_a_type_default_clears_the_previous_one(self, session_client, workspace, default_issue_type):
        other = IssueType.objects.create(workspace=workspace, name="Bug")

        response = session_client.patch(workspace_url(workspace.slug, other.id), {"is_default": True}, format="json")

        assert response.status_code == status.HTTP_200_OK
        default_issue_type.refresh_from_db()
        assert default_issue_type.is_default is False

    @pytest.mark.django_db
    def test_default_type_cannot_be_deleted(self, session_client, workspace, default_issue_type):
        response = session_client.delete(workspace_url(workspace.slug, default_issue_type.id))

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert IssueType.objects.filter(pk=default_issue_type.id).exists()

    @pytest.mark.django_db
    def test_delete_non_default_type(self, session_client, workspace):
        issue_type = IssueType.objects.create(workspace=workspace, name="Bug")

        response = session_client.delete(workspace_url(workspace.slug, issue_type.id))

        assert response.status_code == status.HTTP_204_NO_CONTENT
        assert not IssueType.objects.filter(pk=issue_type.id).exists()

    @pytest.mark.django_db
    def test_member_cannot_create(self, session_client, workspace, create_user):
        workspace.workspace_member.filter(member=create_user).update(role=15)

        response = session_client.post(workspace_url(workspace.slug), {"name": "Bug"}, format="json")

        assert response.status_code == status.HTTP_403_FORBIDDEN


@pytest.mark.contract
class TestProjectIssueTypeEndpoint:
    @pytest.mark.django_db
    def test_list_only_returns_types_enabled_on_the_project(
        self, session_client, workspace, project, default_issue_type
    ):
        IssueType.objects.create(workspace=workspace, name="Not enabled")

        response = session_client.get(project_url(workspace.slug, project.id))

        assert response.status_code == status.HTTP_200_OK
        assert [str(issue_type["id"]) for issue_type in response.data] == [str(default_issue_type.id)]

    @pytest.mark.django_db
    def test_create_type_enables_it_on_the_project(self, session_client, workspace, project, default_issue_type):
        response = session_client.post(project_url(workspace.slug, project.id), {"name": "Bug"}, format="json")

        assert response.status_code == status.HTTP_201_CREATED
        assert response.data["project_ids"] == [str(project.id)]
        assert ProjectIssueType.objects.filter(project=project).count() == 2

    @pytest.mark.django_db
    def test_enable_an_existing_workspace_type(self, session_client, workspace, project, default_issue_type):
        issue_type = IssueType.objects.create(workspace=workspace, name="Bug")

        response = session_client.post(
            project_url(workspace.slug, project.id), {"issue_type_id": str(issue_type.id)}, format="json"
        )

        assert response.status_code == status.HTTP_201_CREATED
        assert ProjectIssueType.objects.filter(project=project, issue_type=issue_type).exists()

    @pytest.mark.django_db
    def test_enabling_the_same_type_twice_is_rejected(self, session_client, workspace, project, default_issue_type):
        response = session_client.post(
            project_url(workspace.slug, project.id), {"issue_type_id": str(default_issue_type.id)}, format="json"
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_disabling_a_type_keeps_it_in_the_workspace(self, session_client, workspace, project, default_issue_type):
        issue_type = IssueType.objects.create(workspace=workspace, name="Bug")
        ProjectIssueType.objects.create(project=project, workspace=workspace, issue_type=issue_type)

        response = session_client.delete(project_url(workspace.slug, project.id, issue_type.id))

        assert response.status_code == status.HTTP_204_NO_CONTENT
        assert IssueType.objects.filter(pk=issue_type.id).exists()
        assert not ProjectIssueType.objects.filter(project=project, issue_type=issue_type).exists()

    @pytest.mark.django_db
    def test_project_default_cannot_be_disabled(self, session_client, workspace, project, default_issue_type):
        response = session_client.delete(project_url(workspace.slug, project.id, default_issue_type.id))

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_update_sets_the_project_default(self, session_client, workspace, project, default_issue_type):
        issue_type = IssueType.objects.create(workspace=workspace, name="Bug")
        ProjectIssueType.objects.create(project=project, workspace=workspace, issue_type=issue_type)

        response = session_client.patch(
            project_url(workspace.slug, project.id, issue_type.id),
            {"is_default": True, "name": "Defect"},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data["name"] == "Defect"
        assert get_default_issue_type(project.id).id == issue_type.id
        assert ProjectIssueType.objects.get(project=project, issue_type=default_issue_type).is_default is False


@pytest.mark.contract
class TestWorkItemTypeAssignment:
    """`type_id` has to be assigned on create and returned in the work item payloads"""

    def issues_url(self, slug, project_id):
        return f"/api/workspaces/{slug}/projects/{project_id}/issues/"

    @pytest.fixture
    def state(self, project):
        return State.objects.create(name="Todo", project=project, workspace=project.workspace, group="unstarted")

    @pytest.mark.django_db
    def test_create_falls_back_to_the_default_type(self, session_client, workspace, project, state, default_issue_type):
        response = session_client.post(
            self.issues_url(workspace.slug, project.id), {"name": "No type given"}, format="json"
        )

        assert response.status_code == status.HTTP_201_CREATED
        assert str(response.data["type_id"]) == str(default_issue_type.id)
        assert Issue.objects.get(pk=response.data["id"]).type_id == default_issue_type.id

    @pytest.mark.django_db
    def test_create_honours_an_explicit_type_id(self, session_client, workspace, project, state, default_issue_type):
        issue_type = IssueType.objects.create(workspace=workspace, name="Bug")
        ProjectIssueType.objects.create(project=project, workspace=workspace, issue_type=issue_type)

        response = session_client.post(
            self.issues_url(workspace.slug, project.id),
            {"name": "Typed", "type_id": str(issue_type.id)},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        assert Issue.objects.get(pk=response.data["id"]).type_id == issue_type.id

    @pytest.mark.django_db
    def test_list_payload_includes_type_id(self, session_client, workspace, project, state, default_issue_type):
        issue = Issue.objects.create(name="Listed", project=project, workspace=workspace, state=state)
        issue.type = default_issue_type
        issue.save()

        response = session_client.get(self.issues_url(workspace.slug, project.id))

        assert response.status_code == status.HTTP_200_OK
        results = response.data["results"] if isinstance(response.data, dict) else response.data
        assert str(results[0]["type_id"]) == str(default_issue_type.id)

# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

import pytest
from rest_framework import status

from plane.db.models import (
    DraftIssue,
    Issue,
    IssueProperty,
    IssuePropertyActivity,
    IssuePropertyOption,
    IssuePropertyValue,
    IssueType,
    Project,
    ProjectIssueType,
    ProjectMember,
    State,
    User,
    WorkspaceMember,
)
from plane.utils.issue_type import get_or_create_default_issue_type


@pytest.fixture
def project(db, workspace, create_user):
    project = Project.objects.create(name="Test Project", identifier="TP", workspace=workspace)
    ProjectMember.objects.create(project=project, member=create_user, role=20, is_active=True)
    return project


@pytest.fixture
def issue_type(project):
    return get_or_create_default_issue_type(project)


@pytest.fixture
def state(project):
    return State.objects.create(name="Todo", project=project, workspace=project.workspace, group="unstarted")


@pytest.fixture
def issue(project, state, issue_type):
    return Issue.objects.create(
        name="Work item", project=project, workspace=project.workspace, state=state, type=issue_type
    )


def make_property(issue_type, **kwargs):
    defaults = {
        "workspace": issue_type.workspace,
        "issue_type": issue_type,
        "name": "severity",
        "display_name": "Severity",
        "property_type": "TEXT",
    }
    return IssueProperty.objects.create(**{**defaults, **kwargs})


def properties_url(slug, issue_type_id, pk=None):
    base_url = f"/api/workspaces/{slug}/issue-types/{issue_type_id}/issue-properties/"
    return f"{base_url}{pk}/" if pk else base_url


def options_url(slug, property_id, pk=None):
    base_url = f"/api/workspaces/{slug}/issue-properties/{property_id}/options/"
    return f"{base_url}{pk}/" if pk else base_url


def values_url(slug, project_id, issue_id):
    return f"/api/workspaces/{slug}/projects/{project_id}/issues/{issue_id}/issue-property-values/"


def bulk_values_url(slug, project_id):
    return f"/api/workspaces/{slug}/projects/{project_id}/issue-property-values/"


def draft_values_url(slug, draft_id):
    return f"/api/workspaces/{slug}/draft-issues/{draft_id}/issue-property-values/"


def make_draft(project, state, issue_type, author, name="Draft"):
    # `created_by` is stamped from the request context, so a draft built straight off
    # the model has to be told who wrote it — drafts are private to their author
    draft = DraftIssue(
        name=name, project=project, workspace=project.workspace, state=state, type=issue_type
    )
    draft.save(created_by_id=author.id)
    return draft


@pytest.mark.contract
class TestIssuePropertyEndpoint:
    @pytest.mark.django_db
    def test_create_property(self, session_client, workspace, issue_type):
        response = session_client.post(
            properties_url(workspace.slug, issue_type.id),
            {"name": "severity", "display_name": "Severity", "property_type": "TEXT"},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        assert str(response.data["issue_type_id"]) == str(issue_type.id)
        assert IssueProperty.objects.filter(issue_type=issue_type, name="severity").exists()

    @pytest.mark.django_db
    def test_create_rejects_duplicate_name_on_the_same_type(self, session_client, workspace, issue_type):
        make_property(issue_type)

        response = session_client.post(
            properties_url(workspace.slug, issue_type.id),
            {"name": "SEVERITY", "display_name": "Severity", "property_type": "TEXT"},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert IssueProperty.objects.filter(issue_type=issue_type).count() == 1

    @pytest.mark.django_db
    def test_the_same_name_is_allowed_on_another_type(self, session_client, workspace, issue_type):
        make_property(issue_type)
        other_type = IssueType.objects.create(workspace=workspace, name="Bug")

        response = session_client.post(
            properties_url(workspace.slug, other_type.id),
            {"name": "severity", "display_name": "Severity", "property_type": "TEXT"},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED

    @pytest.mark.django_db
    def test_relation_property_needs_a_relation_type(self, session_client, workspace, issue_type):
        response = session_client.post(
            properties_url(workspace.slug, issue_type.id),
            {"name": "owner", "display_name": "Owner", "property_type": "RELATION"},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert "relation_type" in response.data

    @pytest.mark.django_db
    def test_relation_type_is_rejected_on_a_non_relation_property(self, session_client, workspace, issue_type):
        response = session_client.post(
            properties_url(workspace.slug, issue_type.id),
            {"name": "owner", "display_name": "Owner", "property_type": "TEXT", "relation_type": "USER"},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_property_type_cannot_be_changed(self, session_client, workspace, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.patch(
            properties_url(workspace.slug, issue_type.id, issue_property.id),
            {"property_type": "DECIMAL"},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        issue_property.refresh_from_db()
        assert issue_property.property_type == "TEXT"

    @pytest.mark.django_db
    def test_list_only_returns_properties_of_the_type(self, session_client, workspace, issue_type):
        issue_property = make_property(issue_type)
        other_type = IssueType.objects.create(workspace=workspace, name="Bug")
        make_property(other_type, name="impact", display_name="Impact")

        response = session_client.get(properties_url(workspace.slug, issue_type.id))

        assert response.status_code == status.HTTP_200_OK
        assert [str(item["id"]) for item in response.data] == [str(issue_property.id)]

    @pytest.mark.django_db
    def test_delete_property(self, session_client, workspace, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.delete(properties_url(workspace.slug, issue_type.id, issue_property.id))

        assert response.status_code == status.HTTP_204_NO_CONTENT
        assert not IssueProperty.objects.filter(pk=issue_property.id).exists()

    @pytest.mark.django_db
    def test_member_cannot_create(self, session_client, workspace, issue_type, create_user):
        workspace.workspace_member.filter(member=create_user).update(role=15)

        response = session_client.post(
            properties_url(workspace.slug, issue_type.id),
            {"name": "severity", "display_name": "Severity", "property_type": "TEXT"},
            format="json",
        )

        assert response.status_code == status.HTTP_403_FORBIDDEN


@pytest.mark.contract
class TestIssuePropertyOptionEndpoint:
    @pytest.fixture
    def option_property(self, issue_type):
        return make_property(issue_type, name="platform", display_name="Platform", property_type="OPTION")

    @pytest.mark.django_db
    def test_create_option(self, session_client, workspace, option_property):
        response = session_client.post(options_url(workspace.slug, option_property.id), {"name": "Web"}, format="json")

        assert response.status_code == status.HTTP_201_CREATED
        assert IssuePropertyOption.objects.filter(property=option_property, name="Web").exists()

    @pytest.mark.django_db
    def test_create_rejects_duplicate_name(self, session_client, workspace, option_property):
        IssuePropertyOption.objects.create(workspace=workspace, property=option_property, name="Web")

        response = session_client.post(options_url(workspace.slug, option_property.id), {"name": "web"}, format="json")

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_marking_an_option_default_clears_the_previous_one(self, session_client, workspace, option_property):
        first = IssuePropertyOption.objects.create(
            workspace=workspace, property=option_property, name="Web", is_default=True
        )

        response = session_client.post(
            options_url(workspace.slug, option_property.id), {"name": "Mobile", "is_default": True}, format="json"
        )

        assert response.status_code == status.HTTP_201_CREATED
        first.refresh_from_db()
        assert first.is_default is False

    @pytest.mark.django_db
    def test_parent_has_to_belong_to_the_same_property(self, session_client, workspace, issue_type, option_property):
        other_property = make_property(issue_type, name="area", display_name="Area", property_type="OPTION")
        foreign_option = IssuePropertyOption.objects.create(
            workspace=workspace, property=other_property, name="Billing"
        )

        response = session_client.post(
            options_url(workspace.slug, option_property.id),
            {"name": "Web", "parent_id": str(foreign_option.id)},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_deleting_an_option_drops_the_values_pointing_at_it(
        self, session_client, workspace, project, issue, option_property
    ):
        option = IssuePropertyOption.objects.create(workspace=workspace, property=option_property, name="Web")
        IssuePropertyValue.objects.create(
            issue=issue, property=option_property, project=project, workspace=workspace, value_uuid=option.id
        )

        response = session_client.delete(options_url(workspace.slug, option_property.id, option.id))

        assert response.status_code == status.HTTP_204_NO_CONTENT
        assert not IssuePropertyValue.objects.filter(property=option_property).exists()


@pytest.mark.contract
class TestIssuePropertyValueEndpoint:
    @pytest.mark.django_db
    def test_read_returns_an_empty_list_per_property(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.get(values_url(workspace.slug, project.id, issue.id))

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): []}

    @pytest.mark.django_db
    def test_replace_text_value(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): ["High"]}
        assert IssuePropertyValue.objects.get(issue=issue, property=issue_property).value_text == "High"

    @pytest.mark.django_db
    def test_replace_overwrites_the_previous_value(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)
        session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["Low"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): ["Low"]}
        assert IssuePropertyValue.objects.filter(issue=issue, property=issue_property).count() == 1

    @pytest.mark.django_db
    def test_replace_leaves_the_properties_it_was_not_given_alone(
        self, session_client, workspace, project, issue, issue_type
    ):
        first = make_property(issue_type)
        second = make_property(issue_type, name="impact", display_name="Impact")
        session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(first.id): ["High"], str(second.id): ["Wide"]}},
            format="json",
        )

        session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(second.id): ["Narrow"]}},
            format="json",
        )

        response = session_client.get(values_url(workspace.slug, project.id, issue.id))
        assert response.data == {str(first.id): ["High"], str(second.id): ["Narrow"]}

    @pytest.mark.django_db
    def test_an_empty_list_clears_the_values(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)
        session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): []}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): []}

    @pytest.mark.django_db
    def test_single_valued_property_rejects_several_values(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High", "Low"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert str(issue_property.id) in response.data

    @pytest.mark.django_db
    def test_multi_valued_property_keeps_every_value(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, is_multi=True)

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High", "Low"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): ["High", "Low"]}

    @pytest.mark.django_db
    def test_required_property_rejects_an_empty_list(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, is_required=True)

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): []}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_decimal_value_is_typed(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="points", display_name="Points", property_type="DECIMAL")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["3.5"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): [3.5]}
        assert IssuePropertyValue.objects.get(issue=issue, property=issue_property).value_decimal == 3.5

    @pytest.mark.django_db
    def test_decimal_value_rejects_text(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="points", display_name="Points", property_type="DECIMAL")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["a lot"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_decimal_value_honours_the_configured_bounds(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(
            issue_type, name="points", display_name="Points", property_type="DECIMAL", settings={"min": 1, "max": 5}
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["9"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_boolean_value_is_typed(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="blocked", display_name="Blocked", property_type="BOOLEAN")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): [True]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): [True]}
        assert IssuePropertyValue.objects.get(issue=issue, property=issue_property).value_boolean is True

    @pytest.mark.django_db
    def test_datetime_value_is_typed(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="due", display_name="Due", property_type="DATETIME")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["2026-08-05T10:00:00Z"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert IssuePropertyValue.objects.get(issue=issue, property=issue_property).value_datetime is not None

    @pytest.mark.django_db
    def test_datetime_value_rejects_garbage(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="due", display_name="Due", property_type="DATETIME")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["not a date"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_url_value_is_validated(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="spec", display_name="Spec", property_type="URL")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["not a url"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_option_value_has_to_be_an_option_of_the_property(
        self, session_client, workspace, project, issue, issue_type
    ):
        issue_property = make_property(issue_type, name="platform", display_name="Platform", property_type="OPTION")
        other_property = make_property(issue_type, name="area", display_name="Area", property_type="OPTION")
        foreign_option = IssuePropertyOption.objects.create(
            workspace=workspace, property=other_property, name="Billing"
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): [str(foreign_option.id)]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_option_value_is_stored_as_the_option_id(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type, name="platform", display_name="Platform", property_type="OPTION")
        option = IssuePropertyOption.objects.create(workspace=workspace, property=issue_property, name="Web")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): [str(option.id)]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): [str(option.id)]}

    @pytest.mark.django_db
    def test_user_relation_has_to_be_a_workspace_member(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(
            issue_type, name="reviewer", display_name="Reviewer", property_type="RELATION", relation_type="USER"
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["8e0d9bb4-1e2f-4a5c-9c2d-4f0a2b6c8d1e"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_user_relation_accepts_a_workspace_member(
        self, session_client, workspace, project, issue, issue_type, create_user
    ):
        issue_property = make_property(
            issue_type, name="reviewer", display_name="Reviewer", property_type="RELATION", relation_type="USER"
        )

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): [str(create_user.id)]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): [str(create_user.id)]}

    @pytest.mark.django_db
    def test_a_property_of_another_type_is_rejected(self, session_client, workspace, project, issue):
        other_type = IssueType.objects.create(workspace=workspace, name="Bug")
        foreign_property = make_property(other_type, name="impact", display_name="Impact")

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(foreign_property.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_guest_cannot_write_values(self, session_client, workspace, project, issue, issue_type, create_user):
        issue_property = make_property(issue_type)
        ProjectMember.objects.filter(project=project, member=create_user).update(role=5)
        # a workspace admin is let through regardless of the project role
        workspace.workspace_member.filter(member=create_user).update(role=15)

        response = session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_403_FORBIDDEN


@pytest.mark.contract
class TestIssuePropertyActivity:
    @pytest.mark.django_db
    def test_setting_a_value_records_a_created_activity(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)

        session_client.post(
            values_url(workspace.slug, project.id, issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        activity = IssuePropertyActivity.objects.get(issue=issue, property=issue_property)
        assert activity.action == "created"
        assert activity.old_value is None
        assert activity.new_value == "High"

    @pytest.mark.django_db
    def test_changing_and_clearing_a_value_records_updated_then_deleted(
        self, session_client, workspace, project, issue, issue_type
    ):
        issue_property = make_property(issue_type)
        url = values_url(workspace.slug, project.id, issue.id)

        session_client.post(url, {"property_values": {str(issue_property.id): ["High"]}}, format="json")
        session_client.post(url, {"property_values": {str(issue_property.id): ["Low"]}}, format="json")
        session_client.post(url, {"property_values": {str(issue_property.id): []}}, format="json")

        actions = list(
            IssuePropertyActivity.objects.filter(issue=issue, property=issue_property)
            .order_by("created_at")
            .values_list("action", flat=True)
        )
        assert actions == ["created", "updated", "deleted"]

    @pytest.mark.django_db
    def test_writing_the_same_value_again_records_nothing(self, session_client, workspace, project, issue, issue_type):
        issue_property = make_property(issue_type)
        url = values_url(workspace.slug, project.id, issue.id)

        session_client.post(url, {"property_values": {str(issue_property.id): ["High"]}}, format="json")
        session_client.post(url, {"property_values": {str(issue_property.id): ["High"]}}, format="json")

        assert IssuePropertyActivity.objects.filter(issue=issue, property=issue_property).count() == 1


@pytest.mark.contract
class TestBulkIssuePropertyValueEndpoint:
    @pytest.mark.django_db
    def test_returns_the_values_of_every_requested_work_item(
        self, session_client, workspace, project, state, issue_type
    ):
        issue_property = make_property(issue_type)
        first = Issue.objects.create(name="First", project=project, workspace=workspace, state=state, type=issue_type)
        second = Issue.objects.create(name="Second", project=project, workspace=workspace, state=state, type=issue_type)
        IssuePropertyValue.objects.create(
            issue=first, property=issue_property, project=project, workspace=workspace, value_text="High"
        )

        response = session_client.get(
            bulk_values_url(workspace.slug, project.id), {"issue_ids": f"{first.id},{second.id}"}
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {
            str(first.id): {str(issue_property.id): ["High"]},
            str(second.id): {str(issue_property.id): []},
        }

    @pytest.mark.django_db
    def test_each_work_item_only_carries_the_properties_of_its_own_type(
        self, session_client, workspace, project, state, issue_type
    ):
        issue_property = make_property(issue_type)
        other_type = IssueType.objects.create(workspace=workspace, name="Bug")
        ProjectIssueType.objects.create(project=project, workspace=workspace, issue_type=other_type)
        other_property = make_property(other_type, name="impact", display_name="Impact")

        typed = Issue.objects.create(name="Typed", project=project, workspace=workspace, state=state, type=issue_type)
        bug = Issue.objects.create(name="Bug", project=project, workspace=workspace, state=state, type=other_type)

        response = session_client.get(
            bulk_values_url(workspace.slug, project.id), {"issue_ids": f"{typed.id},{bug.id}"}
        )

        assert response.status_code == status.HTTP_200_OK
        assert list(response.data[str(typed.id)].keys()) == [str(issue_property.id)]
        assert list(response.data[str(bug.id)].keys()) == [str(other_property.id)]

    @pytest.mark.django_db
    def test_issue_ids_is_required(self, session_client, workspace, project):
        response = session_client.get(bulk_values_url(workspace.slug, project.id))

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_too_many_issue_ids_is_rejected(self, session_client, workspace, project, issue):
        response = session_client.get(
            bulk_values_url(workspace.slug, project.id), {"issue_ids": ",".join([str(issue.id)] * 501)}
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_work_items_of_another_project_are_ignored(
        self, session_client, workspace, project, state, issue_type, create_user
    ):
        make_property(issue_type)
        other_project = Project.objects.create(name="Other", identifier="OTH", workspace=workspace)
        ProjectMember.objects.create(project=other_project, member=create_user, role=20, is_active=True)
        other_state = State.objects.create(name="Todo", project=other_project, workspace=workspace, group="unstarted")
        foreign = Issue.objects.create(
            name="Foreign", project=other_project, workspace=workspace, state=other_state, type=issue_type
        )

        response = session_client.get(bulk_values_url(workspace.slug, project.id), {"issue_ids": str(foreign.id)})

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {}


@pytest.mark.contract
class TestDraftIssuePropertyValueEndpoint:
    """The create modal fills in custom fields before the work item exists, so a draft
    saved out of it keeps them in its own table until it is converted."""

    @pytest.fixture
    def draft_issue(self, project, state, issue_type, create_user):
        return make_draft(project, state, issue_type, create_user)

    @pytest.mark.django_db
    def test_read_returns_an_empty_list_per_property(self, session_client, workspace, draft_issue, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.get(draft_values_url(workspace.slug, draft_issue.id))

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): []}

    @pytest.mark.django_db
    def test_replace_stores_the_value_against_the_draft(self, session_client, workspace, draft_issue, issue_type):
        issue_property = make_property(issue_type)

        response = session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): ["High"]}
        value = IssuePropertyValue.objects.get(draft_issue=draft_issue, property=issue_property)
        assert value.value_text == "High"
        assert value.issue_id is None

    @pytest.mark.django_db
    def test_replace_overwrites_the_previous_value(self, session_client, workspace, draft_issue, issue_type):
        issue_property = make_property(issue_type)
        session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        response = session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["Low"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        assert response.data == {str(issue_property.id): ["Low"]}
        assert IssuePropertyValue.objects.filter(draft_issue=draft_issue, property=issue_property).count() == 1

    @pytest.mark.django_db
    def test_the_same_validation_applies(self, session_client, workspace, draft_issue, issue_type):
        issue_property = make_property(issue_type, property_type="DECIMAL", settings={"max": 5})

        response = session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): [9]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST
        assert str(issue_property.id) in response.data

    @pytest.mark.django_db
    def test_a_draft_is_private_to_whoever_wrote_it(self, session_client, workspace, project, state, issue_type):
        author = User.objects.create(username="other", email="other@example.com")
        WorkspaceMember.objects.create(workspace=workspace, member=author, role=20)
        foreign_draft = make_draft(project, state, issue_type, author, name="Someone else's")

        response = session_client.get(draft_values_url(workspace.slug, foreign_draft.id))

        assert response.status_code == status.HTTP_404_NOT_FOUND

    @pytest.mark.django_db
    def test_a_draft_without_a_project_cannot_hold_values(
        self, session_client, workspace, issue_type, create_user, project
    ):
        issue_property = make_property(issue_type)
        projectless = DraftIssue(name="No project", workspace=workspace, type=issue_type)
        projectless.save(created_by_id=create_user.id)

        response = session_client.post(
            draft_values_url(workspace.slug, projectless.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_a_draft_records_no_activity(self, session_client, workspace, draft_issue, issue_type):
        issue_property = make_property(issue_type)

        session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        assert IssuePropertyActivity.objects.filter(property=issue_property).count() == 0

    @pytest.mark.django_db
    def test_converting_the_draft_carries_the_values_onto_the_work_item(
        self, session_client, workspace, project, draft_issue, issue_type
    ):
        issue_property = make_property(issue_type)
        session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        response = session_client.post(
            f"/api/workspaces/{workspace.slug}/draft-to-issue/{draft_issue.id}/",
            {"name": "Converted", "project_id": str(project.id), "type_id": str(issue_type.id)},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        issue = Issue.objects.get(pk=response.data["id"])
        value = IssuePropertyValue.objects.get(property=issue_property)
        assert value.issue_id == issue.id
        assert value.draft_issue_id is None
        assert value.value_text == "High"

    @pytest.mark.django_db
    def test_converting_to_another_type_drops_the_values_of_the_old_one(
        self, session_client, workspace, project, draft_issue, issue_type
    ):
        issue_property = make_property(issue_type)
        other_type = IssueType.objects.create(workspace=workspace, name="Bug")
        ProjectIssueType.objects.create(project=project, workspace=workspace, issue_type=other_type)
        session_client.post(
            draft_values_url(workspace.slug, draft_issue.id),
            {"property_values": {str(issue_property.id): ["High"]}},
            format="json",
        )

        response = session_client.post(
            f"/api/workspaces/{workspace.slug}/draft-to-issue/{draft_issue.id}/",
            {"name": "Converted", "project_id": str(project.id), "type_id": str(other_type.id)},
            format="json",
        )

        assert response.status_code == status.HTTP_201_CREATED
        assert IssuePropertyValue.objects.filter(property=issue_property, issue__isnull=False).count() == 0

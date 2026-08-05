# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

import json

import pytest
from rest_framework import status

from plane.db.models import (
    Issue,
    IssueProperty,
    IssuePropertyOption,
    IssuePropertyValue,
    Project,
    ProjectMember,
    State,
)
from plane.utils.issue_filters import issue_filters
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


def make_property(issue_type, name="severity", property_type="TEXT", **kwargs):
    return IssueProperty.objects.create(
        workspace=issue_type.workspace,
        issue_type=issue_type,
        name=name,
        display_name=name.title(),
        property_type=property_type,
        **kwargs,
    )


def make_issue(project, state, issue_type, name):
    return Issue.objects.create(name=name, project=project, workspace=project.workspace, state=state, type=issue_type)


def set_value(issue, issue_property, **columns):
    return IssuePropertyValue.objects.create(
        workspace=issue.workspace,
        project=issue.project,
        issue=issue,
        property=issue_property,
        **columns,
    )


def issues_url(slug, project_id):
    return f"/api/workspaces/{slug}/projects/{project_id}/issues/"


def listed_names(response):
    results = response.data["results"]
    return {issue["name"] for issue in results}


@pytest.mark.contract
class TestCustomPropertyRichFilter:
    @pytest.mark.django_db
    def test_text_property_is_filtered_by_exact_value(self, session_client, workspace, project, state, issue_type):
        severity = make_property(issue_type)
        matching = make_issue(project, state, issue_type, "Matching")
        other = make_issue(project, state, issue_type, "Other")
        set_value(matching, severity, value_text="blocker")
        set_value(other, severity, value_text="trivial")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{severity.id}__exact": "blocker"})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Matching"}

    @pytest.mark.django_db
    def test_text_property_is_filtered_by_substring(self, session_client, workspace, project, state, issue_type):
        severity = make_property(issue_type)
        matching = make_issue(project, state, issue_type, "Matching")
        other = make_issue(project, state, issue_type, "Other")
        set_value(matching, severity, value_text="Hard Blocker")
        set_value(other, severity, value_text="trivial")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{severity.id}__icontains": "blocK"})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Matching"}

    @pytest.mark.django_db
    def test_option_property_is_filtered_by_a_list_of_options(
        self, session_client, workspace, project, state, issue_type
    ):
        component = make_property(issue_type, name="component", property_type="OPTION")
        api = IssuePropertyOption.objects.create(workspace=workspace, property=component, name="api")
        web = IssuePropertyOption.objects.create(workspace=workspace, property=component, name="web")
        live = IssuePropertyOption.objects.create(workspace=workspace, property=component, name="live")
        on_api = make_issue(project, state, issue_type, "On api")
        on_web = make_issue(project, state, issue_type, "On web")
        on_live = make_issue(project, state, issue_type, "On live")
        set_value(on_api, component, value_uuid=api.id)
        set_value(on_web, component, value_uuid=web.id)
        set_value(on_live, component, value_uuid=live.id)

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{component.id}__in": [str(api.id), str(web.id)]})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"On api", "On web"}

    @pytest.mark.django_db
    def test_a_multi_valued_property_matches_on_any_of_its_values(
        self, session_client, workspace, project, state, issue_type
    ):
        component = make_property(issue_type, name="component", property_type="OPTION", is_multi=True)
        api = IssuePropertyOption.objects.create(workspace=workspace, property=component, name="api")
        web = IssuePropertyOption.objects.create(workspace=workspace, property=component, name="web")
        both = make_issue(project, state, issue_type, "Both")
        set_value(both, component, value_uuid=api.id)
        set_value(both, component, value_uuid=web.id)

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{component.id}__exact": str(web.id)})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Both"}

    @pytest.mark.django_db
    def test_decimal_property_is_filtered_by_range(self, session_client, workspace, project, state, issue_type):
        story_points = make_property(issue_type, name="points", property_type="DECIMAL")
        small = make_issue(project, state, issue_type, "Small")
        large = make_issue(project, state, issue_type, "Large")
        set_value(small, story_points, value_decimal=3)
        set_value(large, story_points, value_decimal=13)

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{story_points.id}__range": ["1", "8"]})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Small"}

    @pytest.mark.django_db
    def test_boolean_property_is_filtered_by_exact_value(self, session_client, workspace, project, state, issue_type):
        regression = make_property(issue_type, name="regression", property_type="BOOLEAN")
        yes = make_issue(project, state, issue_type, "Yes")
        no = make_issue(project, state, issue_type, "No")
        set_value(yes, regression, value_boolean=True)
        set_value(no, regression, value_boolean=False)

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{regression.id}__exact": "true"})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Yes"}

    @pytest.mark.django_db
    def test_datetime_property_matches_on_the_calendar_day(self, session_client, workspace, project, state, issue_type):
        released = make_property(issue_type, name="released_on", property_type="DATETIME")
        on_day = make_issue(project, state, issue_type, "On day")
        next_day = make_issue(project, state, issue_type, "Next day")
        set_value(on_day, released, value_datetime="2026-03-04T17:45:00Z")
        set_value(next_day, released, value_datetime="2026-03-05T09:00:00Z")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{released.id}__exact": "2026-03-04"})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"On day"}

    @pytest.mark.django_db
    def test_two_property_conditions_are_combined_with_and(self, session_client, workspace, project, state, issue_type):
        severity = make_property(issue_type)
        story_points = make_property(issue_type, name="points", property_type="DECIMAL")
        both = make_issue(project, state, issue_type, "Both")
        only_severity = make_issue(project, state, issue_type, "Only severity")
        set_value(both, severity, value_text="blocker")
        set_value(both, story_points, value_decimal=3)
        set_value(only_severity, severity, value_text="blocker")
        set_value(only_severity, story_points, value_decimal=13)

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {
                "filters": json.dumps(
                    {
                        "and": [
                            {f"property_{severity.id}__exact": "blocker"},
                            {f"property_{story_points.id}__exact": "3"},
                        ]
                    }
                )
            },
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Both"}

    @pytest.mark.django_db
    def test_a_property_condition_combines_with_a_built_in_one(
        self, session_client, workspace, project, state, issue_type
    ):
        severity = make_property(issue_type)
        urgent = make_issue(project, state, issue_type, "Urgent")
        urgent.priority = "urgent"
        urgent.save()
        low = make_issue(project, state, issue_type, "Low")
        set_value(urgent, severity, value_text="blocker")
        set_value(low, severity, value_text="blocker")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {
                "filters": json.dumps(
                    {"and": [{f"property_{severity.id}__exact": "blocker"}, {"priority__exact": "urgent"}]}
                )
            },
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Urgent"}

    @pytest.mark.django_db
    def test_not_returns_the_work_items_without_a_matching_value(
        self, session_client, workspace, project, state, issue_type
    ):
        severity = make_property(issue_type)
        blocker = make_issue(project, state, issue_type, "Blocker")
        trivial = make_issue(project, state, issue_type, "Trivial")
        unset = make_issue(project, state, issue_type, "Unset")
        set_value(blocker, severity, value_text="blocker")
        set_value(trivial, severity, value_text="trivial")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({"not": {f"property_{severity.id}__exact": "blocker"}})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Trivial", "Unset"}
        assert unset.name in listed_names(response)

    @pytest.mark.django_db
    def test_a_soft_deleted_value_no_longer_matches(self, session_client, workspace, project, state, issue_type):
        severity = make_property(issue_type)
        issue = make_issue(project, state, issue_type, "Matching")
        value = set_value(issue, severity, value_text="blocker")
        value.delete()

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{severity.id}__exact": "blocker"})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == set()

    @pytest.mark.django_db
    def test_an_unknown_property_is_rejected(self, session_client, workspace, project, state, issue_type):
        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({"property_2f8f6f3e-1111-2222-3333-444455556666__exact": "blocker"})},
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_a_value_of_the_wrong_type_is_rejected(self, session_client, workspace, project, state, issue_type):
        story_points = make_property(issue_type, name="points", property_type="DECIMAL")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({f"property_{story_points.id}__exact": "not-a-number"})},
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_a_built_in_list_lookup_keeps_every_value(self, session_client, workspace, project, state, issue_type):
        # a JSON list is serialised into the filterset's CSV field, so an `__in` lookup
        # written as an array matches on all of its values rather than on the last one
        urgent = make_issue(project, state, issue_type, "Urgent")
        urgent.priority = "urgent"
        urgent.save()
        high = make_issue(project, state, issue_type, "High")
        high.priority = "high"
        high.save()
        make_issue(project, state, issue_type, "Low")

        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({"priority__in": ["urgent", "high"]})},
        )

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Urgent", "High"}

    @pytest.mark.django_db
    def test_a_key_that_is_not_a_property_is_still_rejected(self, session_client, workspace, project):
        response = session_client.get(
            issues_url(workspace.slug, project.id),
            {"filters": json.dumps({"description_html__exact": "secret"})},
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST


@pytest.mark.contract
class TestCustomPropertyLegacyFilter:
    @pytest.mark.django_db
    def test_a_property_query_parameter_filters_the_list(self, session_client, workspace, project, state, issue_type):
        severity = make_property(issue_type)
        matching = make_issue(project, state, issue_type, "Matching")
        other = make_issue(project, state, issue_type, "Other")
        set_value(matching, severity, value_text="blocker")
        set_value(other, severity, value_text="trivial")

        response = session_client.get(issues_url(workspace.slug, project.id), {f"property_{severity.id}": "blocker"})

        assert response.status_code == status.HTTP_200_OK
        assert listed_names(response) == {"Matching"}

    @pytest.mark.django_db
    def test_two_property_query_parameters_are_combined_with_and(self, db, project, state, issue_type):
        severity = make_property(issue_type)
        story_points = make_property(issue_type, name="points", property_type="DECIMAL")
        both = make_issue(project, state, issue_type, "Both")
        only_severity = make_issue(project, state, issue_type, "Only severity")
        set_value(both, severity, value_text="blocker")
        set_value(both, story_points, value_decimal=3)
        set_value(only_severity, severity, value_text="blocker")
        set_value(only_severity, story_points, value_decimal=13)

        filters = issue_filters({f"property_{severity.id}": "blocker", f"property_{story_points.id}": "3"}, "GET")

        assert set(Issue.issue_objects.filter(**filters).values_list("name", flat=True)) == {"Both"}

    @pytest.mark.django_db
    def test_a_prefixed_filter_targets_the_related_work_item(self, db, project, state, issue_type):
        severity = make_property(issue_type)
        issue = make_issue(project, state, issue_type, "Matching")
        set_value(issue, severity, value_text="blocker")

        filters = issue_filters({f"property_{severity.id}": "blocker"}, "GET", prefix="issue__")

        assert list(filters) == ["issue__pk__in"]

    @pytest.mark.django_db
    def test_an_unrelated_query_parameter_adds_no_filter(self, db, issue_type):
        assert issue_filters({"property_not_a_uuid": "blocker"}, "GET") == {}

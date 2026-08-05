# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

"""Custom field values in the work item export."""

import csv
import json
from io import StringIO

import pytest

from plane.db.models import (
    Issue,
    IssueProperty,
    IssuePropertyOption,
    IssuePropertyValue,
    Project,
    ProjectMember,
    State,
)
from plane.utils.issue_property import property_values_index
from plane.utils.issue_type import get_or_create_default_issue_type
from plane.utils.porters.exporter import DataExporter
from plane.utils.porters.serializers.issue import IssueExportSerializer


@pytest.fixture
def project(db, workspace, create_user):
    project = Project.objects.create(name="Test Project", identifier="TP", workspace=workspace, created_by=create_user)
    ProjectMember.objects.create(project=project, member=create_user, role=20, is_active=True)
    return project


@pytest.fixture
def issue(project):
    state = State.objects.create(name="Todo", project=project, workspace=project.workspace, group="unstarted")
    return Issue.objects.create(
        name="Work item",
        project=project,
        workspace=project.workspace,
        state=state,
        type=get_or_create_default_issue_type(project),
    )


def make_property(issue, **kwargs):
    defaults = {
        "workspace": issue.workspace,
        "issue_type": issue.type,
        "name": "severity",
        "display_name": "Severity",
        "property_type": "TEXT",
    }
    return IssueProperty.objects.create(**{**defaults, **kwargs})


def set_value(issue, issue_property, **value):
    return IssuePropertyValue.objects.create(
        issue=issue,
        property=issue_property,
        project=issue.project,
        workspace=issue.workspace,
        **value,
    )


def export(issues, provider="json"):
    index = property_values_index(issues.values_list("id", flat=True))
    exporter = DataExporter(IssueExportSerializer, format_type=provider, context={"property_values_index": index})
    return exporter.export("export", issues)[1]


@pytest.mark.django_db
class TestPropertyValuesIndex:
    def test_values_are_filed_under_the_display_name(self, issue):
        issue_property = make_property(issue)
        set_value(issue, issue_property, value_text="High")

        index = property_values_index([issue.id])

        assert index == {str(issue.id): {"Severity": ["High"]}}

    def test_an_option_is_rendered_by_its_name(self, issue):
        issue_property = make_property(issue, name="priority", display_name="Priority", property_type="OPTION")
        option = IssuePropertyOption.objects.create(
            workspace=issue.workspace, property=issue_property, name="Blocker", sort_order=1
        )
        set_value(issue, issue_property, value_uuid=option.id)

        index = property_values_index([issue.id])

        assert index == {str(issue.id): {"Priority": ["Blocker"]}}

    def test_a_relation_is_rendered_as_the_work_item_identifier(self, issue, project):
        related = Issue.objects.create(name="Other", project=project, workspace=project.workspace, type=issue.type)
        issue_property = make_property(
            issue, name="blocker", display_name="Blocker", property_type="RELATION", relation_type="ISSUE"
        )
        set_value(issue, issue_property, value_uuid=related.id)

        index = property_values_index([issue.id])

        assert index == {str(issue.id): {"Blocker": [f"TP-{related.sequence_id}"]}}

    def test_a_multi_valued_field_keeps_every_value(self, issue):
        issue_property = make_property(issue, is_multi=True)
        set_value(issue, issue_property, value_text="High")
        set_value(issue, issue_property, value_text="Low")

        index = property_values_index([issue.id])

        assert index == {str(issue.id): {"Severity": ["High", "Low"]}}

    def test_the_webhook_shape_keys_by_api_name_and_keeps_ids(self, issue):
        issue_property = make_property(issue, name="priority", display_name="Priority", property_type="OPTION")
        option = IssuePropertyOption.objects.create(
            workspace=issue.workspace, property=issue_property, name="Blocker", sort_order=1
        )
        set_value(issue, issue_property, value_uuid=option.id)

        index = property_values_index([issue.id], key="name", as_labels=False)

        assert index == {str(issue.id): {"priority": [str(option.id)]}}


@pytest.mark.django_db
class TestIssueExport:
    def test_the_json_export_carries_the_values(self, issue):
        issue_property = make_property(issue)
        set_value(issue, issue_property, value_text="High")

        rows = json.loads(export(Issue.objects.filter(pk=issue.id)))

        assert rows[0]["property_values"] == {"Severity": ["High"]}
        assert rows[0]["type_name"] == issue.type.name

    def test_the_csv_export_gives_each_field_its_own_column(self, issue):
        issue_property = make_property(issue)
        set_value(issue, issue_property, value_text="High")

        content = export(Issue.objects.filter(pk=issue.id), provider="csv")

        rows = list(csv.DictReader(StringIO(content)))
        assert rows[0]["Property Values  Severity"] == '["High"]'

    def test_a_work_item_without_values_exports_an_empty_map(self, issue):
        make_property(issue)

        rows = json.loads(export(Issue.objects.filter(pk=issue.id)))

        assert rows[0]["property_values"] == {}

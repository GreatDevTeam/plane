# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Module imports
from plane.db.models import IssueType, ProjectIssueType

DEFAULT_ISSUE_TYPE_NAME = "Task"
DEFAULT_ISSUE_TYPE_DESCRIPTION = "The default work item type."
DEFAULT_ISSUE_TYPE_LOGO_PROPS = {
    "in_use": "icon",
    "icon": {"name": "layers", "color": "#6695FF", "background_color": "#6695FF20"},
}


def get_or_create_default_issue_type(project, created_by_id=None):
    """Return the workspace's default work item type, enabling it on the project.

    There is a single default type per workspace — the ``project_issue_types`` join is
    what makes it available inside a project, which is also the shape the public API
    serializer already queries for (``project_issue_types__project_id`` + ``is_default``).
    """
    issue_type = IssueType.objects.filter(workspace_id=project.workspace_id, is_default=True, is_epic=False).first()

    if issue_type is None:
        issue_type = IssueType(
            workspace_id=project.workspace_id,
            name=DEFAULT_ISSUE_TYPE_NAME,
            description=DEFAULT_ISSUE_TYPE_DESCRIPTION,
            logo_props=DEFAULT_ISSUE_TYPE_LOGO_PROPS,
            is_default=True,
            is_active=True,
            level=0,
        )
        issue_type.save(created_by_id=created_by_id)

    if not ProjectIssueType.objects.filter(project=project, issue_type=issue_type).exists():
        project_issue_type = ProjectIssueType(project=project, issue_type=issue_type, level=0, is_default=True)
        project_issue_type.save(created_by_id=created_by_id)

    return issue_type


def get_default_issue_type(project_id):
    """Return the work item type new work items of this project fall back to.

    The per project flag wins so a project can override the workspace wide default; the
    workspace default is the fallback for projects that never set one.
    """
    enabled_types = IssueType.objects.filter(
        project_issue_types__project_id=project_id,
        project_issue_types__deleted_at__isnull=True,
    )
    return enabled_types.filter(project_issue_types__is_default=True).first() or enabled_types.filter(
        is_default=True
    ).first()

# Seeds a default work item type per workspace, enables it on every project and
# backfills the type of the work items that were created before types existed.

from django.db import migrations

# Frozen copies of the defaults in plane/utils/issue_type.py — a migration must keep
# writing the same values even if the runtime defaults change later.
DEFAULT_ISSUE_TYPE_NAME = "Task"
DEFAULT_ISSUE_TYPE_DESCRIPTION = "The default work item type."
DEFAULT_ISSUE_TYPE_LOGO_PROPS = {
    "in_use": "icon",
    "icon": {"name": "layers", "color": "#6695FF", "background_color": "#6695FF20"},
}


def seed_default_issue_types(apps, schema_editor):
    Project = apps.get_model("db", "Project")
    IssueType = apps.get_model("db", "IssueType")
    ProjectIssueType = apps.get_model("db", "ProjectIssueType")
    Issue = apps.get_model("db", "Issue")
    DraftIssue = apps.get_model("db", "DraftIssue")

    workspace_ids = (
        Project.objects.filter(deleted_at__isnull=True).values_list("workspace_id", flat=True).distinct()
    )

    for workspace_id in workspace_ids.iterator():
        issue_type = IssueType.objects.filter(
            workspace_id=workspace_id, is_default=True, is_epic=False, deleted_at__isnull=True
        ).first()

        if issue_type is None:
            issue_type = IssueType.objects.create(
                workspace_id=workspace_id,
                name=DEFAULT_ISSUE_TYPE_NAME,
                description=DEFAULT_ISSUE_TYPE_DESCRIPTION,
                logo_props=DEFAULT_ISSUE_TYPE_LOGO_PROPS,
                is_default=True,
                is_active=True,
                level=0,
            )

        project_ids = list(
            Project.objects.filter(workspace_id=workspace_id, deleted_at__isnull=True).values_list("id", flat=True)
        )
        enabled_project_ids = set(
            ProjectIssueType.objects.filter(project_id__in=project_ids, deleted_at__isnull=True).values_list(
                "project_id", flat=True
            )
        )

        ProjectIssueType.objects.bulk_create(
            [
                ProjectIssueType(
                    project_id=project_id,
                    workspace_id=workspace_id,
                    issue_type=issue_type,
                    level=0,
                    is_default=True,
                )
                for project_id in project_ids
                if project_id not in enabled_project_ids
            ],
            batch_size=500,
        )

        # `_base_manager` rather than the model managers: the backfill has to reach archived,
        # draft and soft deleted work items too, and the historical Issue model only carries
        # `issue_objects`, which filters all of those out.
        Issue._base_manager.filter(workspace_id=workspace_id, type__isnull=True).update(type=issue_type)
        DraftIssue._base_manager.filter(workspace_id=workspace_id, type__isnull=True).update(type=issue_type)


class Migration(migrations.Migration):
    dependencies = [("db", "0121_alter_estimate_type")]

    operations = [migrations.RunPython(seed_default_issue_types, migrations.RunPython.noop)]

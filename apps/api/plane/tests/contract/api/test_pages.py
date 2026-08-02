# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

from unittest import mock

import pytest
from rest_framework import status

from plane.db.models import Page, Project, ProjectMember, ProjectPage
from plane.utils.live_server import LiveServerUnavailable


@pytest.fixture
def project(db, workspace, create_user):
    project = Project.objects.create(
        name="Test Project",
        identifier="TP",
        workspace=workspace,
        created_by=create_user,
    )
    ProjectMember.objects.create(
        project=project,
        member=create_user,
        role=20,
        is_active=True,
    )
    return project


@pytest.fixture
def other_project(db, workspace, create_user):
    project = Project.objects.create(
        name="Other Project",
        identifier="OP",
        workspace=workspace,
        created_by=create_user,
    )
    ProjectMember.objects.create(
        project=project,
        member=create_user,
        role=20,
        is_active=True,
    )
    return project


@pytest.fixture
def create_page(db, workspace, project, create_user):
    def _create(name, description_html="<p></p>", parent=None):
        page = Page.objects.create(
            name=name,
            description_html=description_html,
            workspace=workspace,
            owned_by=create_user,
            access=Page.PUBLIC_ACCESS,
            parent=parent,
        )
        ProjectPage.objects.create(
            project=project,
            page=page,
            workspace=workspace,
        )
        return page

    return _create


@pytest.fixture
def create_page_in_project(db, workspace, create_user):
    def _create(proj, name, parent=None):
        page = Page.objects.create(
            name=name,
            description_html="<p></p>",
            workspace=workspace,
            owned_by=create_user,
            access=Page.PUBLIC_ACCESS,
            parent=parent,
        )
        ProjectPage.objects.create(
            project=proj,
            page=page,
            workspace=workspace,
        )
        return page

    return _create


@pytest.mark.contract
class TestPageSearchAPIEndpoint:
    def get_search_url(self, workspace_slug, project_id):
        return f"/api/v1/workspaces/{workspace_slug}/projects/{project_id}/pages/search/"

    @pytest.mark.django_db
    def test_search_by_name(self, api_key_client, workspace, project, create_page):
        create_page("Alpha Page")
        create_page("Beta Page")

        url = self.get_search_url(workspace.slug, project.id)
        response = api_key_client.get(url, {"search": "Alpha"})

        assert response.status_code == status.HTTP_200_OK
        assert len(response.data) == 1
        assert response.data[0]["name"] == "Alpha Page"

    @pytest.mark.django_db
    def test_search_by_description(self, api_key_client, workspace, project, create_page):
        create_page("Page One", "<p>unique description keyword</p>")
        create_page("Page Two", "<p>something else</p>")

        url = self.get_search_url(workspace.slug, project.id)
        response = api_key_client.get(url, {"search": "unique description keyword"})

        assert response.status_code == status.HTTP_200_OK
        assert len(response.data) == 1
        assert response.data[0]["name"] == "Page One"

    @pytest.mark.django_db
    def test_search_matches_name_or_description(self, api_key_client, workspace, project, create_page):
        create_page("Alpha Page", "<p>some content</p>")
        create_page("Other Page", "<p>alpha content here</p>")
        create_page("Unrelated Page", "<p>nothing matches</p>")

        url = self.get_search_url(workspace.slug, project.id)
        response = api_key_client.get(url, {"search": "alpha"})

        assert response.status_code == status.HTTP_200_OK
        assert len(response.data) == 2
        names = {p["name"] for p in response.data}
        assert names == {"Alpha Page", "Other Page"}

    @pytest.mark.django_db
    def test_search_empty_returns_all(self, api_key_client, workspace, project, create_page):
        create_page("Page One")
        create_page("Page Two")

        url = self.get_search_url(workspace.slug, project.id)
        response = api_key_client.get(url)

        assert response.status_code == status.HTTP_200_OK
        assert len(response.data) == 2


@pytest.mark.contract
class TestPageParentIdSupport:
    def get_list_url(self, workspace_slug, project_id):
        return f"/api/v1/workspaces/{workspace_slug}/projects/{project_id}/pages/"

    def get_detail_url(self, workspace_slug, project_id, page_id):
        return f"/api/v1/workspaces/{workspace_slug}/projects/{project_id}/pages/{page_id}/"

    @pytest.mark.django_db
    def test_create_page_with_valid_parent(self, api_key_client, workspace, project, create_page):
        parent = create_page("Parent Page")
        url = self.get_list_url(workspace.slug, project.id)

        response = api_key_client.post(url, {"name": "Child Page", "parent": str(parent.id)}, format="json")

        assert response.status_code == status.HTTP_201_CREATED
        assert str(response.data["parent"]) == str(parent.id)

    @pytest.mark.django_db
    def test_create_page_with_parent_id_alias(self, api_key_client, workspace, project, create_page):
        parent = create_page("Parent Page")
        url = self.get_list_url(workspace.slug, project.id)

        response = api_key_client.post(url, {"name": "Child Page", "parent_id": str(parent.id)}, format="json")

        assert response.status_code == status.HTTP_201_CREATED
        assert str(response.data["parent"]) == str(parent.id)

    @pytest.mark.django_db
    def test_create_page_with_parent_from_different_project_rejected(
        self, api_key_client, workspace, project, other_project, create_page_in_project
    ):
        other_parent = create_page_in_project(other_project, "Other Project Page")
        url = self.get_list_url(workspace.slug, project.id)

        response = api_key_client.post(url, {"name": "Child Page", "parent": str(other_parent.id)}, format="json")

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_create_page_with_nonexistent_parent_rejected(self, api_key_client, workspace, project):
        import uuid
        url = self.get_list_url(workspace.slug, project.id)

        response = api_key_client.post(
            url, {"name": "Child Page", "parent_id": str(uuid.uuid4())}, format="json"
        )

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_update_page_to_valid_parent(self, api_key_client, workspace, project, create_page):
        parent = create_page("Parent Page")
        child = create_page("Child Page")
        url = self.get_detail_url(workspace.slug, project.id, child.id)

        response = api_key_client.patch(url, {"parent": str(parent.id)}, format="json")

        assert response.status_code == status.HTTP_200_OK
        assert str(response.data["parent"]) == str(parent.id)

    @pytest.mark.django_db
    def test_update_page_to_create_cycle_rejected(self, api_key_client, workspace, project, create_page):
        page_a = create_page("Page A")
        page_b = create_page("Page B", parent=page_a)
        url = self.get_detail_url(workspace.slug, project.id, page_a.id)

        # Trying to make A a child of B would create A -> B -> A
        response = api_key_client.patch(url, {"parent": str(page_b.id)}, format="json")

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_update_page_to_self_parent_rejected(self, api_key_client, workspace, project, create_page):
        page = create_page("Page")
        url = self.get_detail_url(workspace.slug, project.id, page.id)

        response = api_key_client.patch(url, {"parent": str(page.id)}, format="json")

        assert response.status_code == status.HTTP_400_BAD_REQUEST

    @pytest.mark.django_db
    def test_list_pages_default_returns_top_level(self, api_key_client, workspace, project, create_page):
        parent = create_page("Parent Page")
        create_page("Child Page", parent=parent)

        url = self.get_list_url(workspace.slug, project.id)
        response = api_key_client.get(url)

        assert response.status_code == status.HTTP_200_OK
        page_ids = [str(p["id"]) for p in response.data["results"]]
        assert str(parent.id) in page_ids
        # Child should not appear in top-level listing
        child_in_results = any(p["parent"] is not None for p in response.data["results"])
        assert not child_in_results

    @pytest.mark.django_db
    def test_list_pages_filter_by_parent_id_null_returns_top_level(
        self, api_key_client, workspace, project, create_page
    ):
        parent = create_page("Parent Page")
        create_page("Child Page", parent=parent)

        url = self.get_list_url(workspace.slug, project.id)
        response = api_key_client.get(url, {"parent_id": "null"})

        assert response.status_code == status.HTTP_200_OK
        page_ids = [str(p["id"]) for p in response.data["results"]]
        assert str(parent.id) in page_ids

    @pytest.mark.django_db
    def test_list_pages_filter_by_parent_id_returns_children(
        self, api_key_client, workspace, project, create_page
    ):
        parent = create_page("Parent Page")
        child = create_page("Child Page", parent=parent)
        create_page("Other Top-Level Page")

        url = self.get_list_url(workspace.slug, project.id)
        response = api_key_client.get(url, {"parent_id": str(parent.id)})

        assert response.status_code == status.HTTP_200_OK
        page_ids = [str(p["id"]) for p in response.data["results"]]
        assert str(child.id) in page_ids
        assert str(parent.id) not in page_ids

    @pytest.mark.django_db
    def test_page_response_includes_parent_field(self, api_key_client, workspace, project, create_page):
        parent = create_page("Parent Page")
        child = create_page("Child Page", parent=parent)

        url = self.get_detail_url(workspace.slug, project.id, child.id)
        response = api_key_client.get(url)

        assert response.status_code == status.HTTP_200_OK
        assert "parent" in response.data
        assert str(response.data["parent"]) == str(parent.id)


@pytest.mark.contract
class TestPageUpdateRewritesCollaborativeSnapshot:
    """
    The editor renders a page from its collaborative (Yjs) snapshot in
    description_binary, never from description_html, and the page title is part
    of that snapshot too. An API update therefore has to rewrite the snapshot
    through the live server for the change to be visible in the UI.
    """

    def get_detail_url(self, workspace_slug, project_id, page_id):
        return f"/api/v1/workspaces/{workspace_slug}/projects/{project_id}/pages/{page_id}/"

    @pytest.fixture
    def page_with_snapshot(self, create_page):
        page = create_page("Page With Snapshot", "<p>old content</p>")
        page.description_binary = b"stale-binary"
        page.description_json = {"type": "doc", "content": []}
        page.save()
        return page

    @pytest.fixture
    def live_server_call(self):
        """Stub the live server, returning the snapshot it would have built."""
        with mock.patch("plane.api.serializers.page.replace_document_content") as patched:
            patched.return_value = {
                "description_binary": b"rewritten-binary",
                "description_html": "<p>new content</p>",
                "description_json": {"type": "doc", "content": [{"type": "paragraph"}]},
            }
            yield patched

    @pytest.mark.django_db
    def test_update_description_html_rewrites_snapshot(
        self, api_key_client, workspace, project, page_with_snapshot, live_server_call
    ):
        url = self.get_detail_url(workspace.slug, project.id, page_with_snapshot.id)

        response = api_key_client.patch(url, {"description_html": "<p>new content</p>"}, format="json")

        assert response.status_code == status.HTTP_200_OK
        # the existing snapshot is handed over so the rewrite is an update of it, not a new document,
        # and the name is left out so the rewrite does not revert a rename made in the editor
        live_server_call.assert_called_once_with(
            document_id=page_with_snapshot.id,
            description_html="<p>new content</p>",
            name=None,
            description_binary=b"stale-binary",
        )
        page_with_snapshot.refresh_from_db()
        assert page_with_snapshot.description_html == "<p>new content</p>"
        assert bytes(page_with_snapshot.description_binary) == b"rewritten-binary"
        assert page_with_snapshot.description_json == {"type": "doc", "content": [{"type": "paragraph"}]}

    @pytest.mark.django_db
    def test_update_name_rewrites_snapshot(
        self, api_key_client, workspace, project, page_with_snapshot, live_server_call
    ):
        # The page title is part of the snapshot, so a rename has to go through it too
        url = self.get_detail_url(workspace.slug, project.id, page_with_snapshot.id)

        response = api_key_client.patch(url, {"name": "Renamed Page"}, format="json")

        assert response.status_code == status.HTTP_200_OK
        # the body is left out: rewriting it with the stored HTML would revert unsaved edits
        live_server_call.assert_called_once_with(
            document_id=page_with_snapshot.id,
            description_html=None,
            name="Renamed Page",
            description_binary=b"stale-binary",
        )
        page_with_snapshot.refresh_from_db()
        assert page_with_snapshot.name == "Renamed Page"
        assert bytes(page_with_snapshot.description_binary) == b"rewritten-binary"

    @pytest.mark.django_db
    def test_page_is_left_untouched_when_live_server_is_unavailable(
        self, api_key_client, workspace, project, page_with_snapshot, live_server_call
    ):
        # Rebuilding the snapshot from HTML produces a Yjs document unrelated to the one the
        # clients hold, and Yjs merges documents instead of replacing them, so every browser
        # that cached the page would end up showing the old and the new content at once. The
        # update is refused instead, and the page stays as it was.
        live_server_call.side_effect = LiveServerUnavailable("the live server is not configured")
        url = self.get_detail_url(workspace.slug, project.id, page_with_snapshot.id)

        response = api_key_client.patch(url, {"description_html": "<p>new content</p>"}, format="json")

        assert response.status_code == status.HTTP_503_SERVICE_UNAVAILABLE
        assert "the live server is not configured" in response.data["error"]
        page_with_snapshot.refresh_from_db()
        assert page_with_snapshot.description_html == "<p>old content</p>"
        assert bytes(page_with_snapshot.description_binary) == b"stale-binary"
        assert page_with_snapshot.description_json == {"type": "doc", "content": []}

    @pytest.mark.django_db
    def test_page_without_a_snapshot_does_not_need_the_live_server(
        self, api_key_client, workspace, project, create_page, live_server_call
    ):
        # A page that was never opened in the editor has no snapshot for the clients to hold:
        # the live server builds one from description_html the first time it is opened.
        page = create_page("Page Without Snapshot", "<p>old content</p>")
        live_server_call.side_effect = LiveServerUnavailable("the live server is not configured")
        url = self.get_detail_url(workspace.slug, project.id, page.id)

        response = api_key_client.patch(url, {"description_html": "<p>new content</p>"}, format="json")

        assert response.status_code == status.HTTP_200_OK
        live_server_call.assert_not_called()
        page.refresh_from_db()
        assert page.description_html == "<p>new content</p>"

    @pytest.mark.django_db
    def test_update_unrelated_field_keeps_snapshot(
        self, api_key_client, workspace, project, page_with_snapshot, live_server_call
    ):
        url = self.get_detail_url(workspace.slug, project.id, page_with_snapshot.id)

        response = api_key_client.patch(url, {"color": "#ff0000"}, format="json")

        assert response.status_code == status.HTTP_200_OK
        live_server_call.assert_not_called()
        page_with_snapshot.refresh_from_db()
        assert bytes(page_with_snapshot.description_binary) == b"stale-binary"

    @pytest.mark.django_db
    def test_update_with_unchanged_values_keeps_snapshot(
        self, api_key_client, workspace, project, page_with_snapshot, live_server_call
    ):
        url = self.get_detail_url(workspace.slug, project.id, page_with_snapshot.id)

        response = api_key_client.patch(
            url,
            {"name": page_with_snapshot.name, "description_html": page_with_snapshot.description_html},
            format="json",
        )

        assert response.status_code == status.HTTP_200_OK
        live_server_call.assert_not_called()
        page_with_snapshot.refresh_from_db()
        assert bytes(page_with_snapshot.description_binary) == b"stale-binary"

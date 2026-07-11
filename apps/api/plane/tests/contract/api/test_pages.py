# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

import pytest
from rest_framework import status

from plane.db.models import Page, Project, ProjectMember, ProjectPage


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

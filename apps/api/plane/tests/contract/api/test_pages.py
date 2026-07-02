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
def create_page(db, workspace, project, create_user):
    def _create(name, description_html="<p></p>"):
        page = Page.objects.create(
            name=name,
            description_html=description_html,
            workspace=workspace,
            owned_by=create_user,
            access=Page.PUBLIC_ACCESS,
        )
        ProjectPage.objects.create(
            project=project,
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

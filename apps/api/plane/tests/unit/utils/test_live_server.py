# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

import base64
import uuid
from unittest import mock

import pytest
import requests

from plane.utils.live_server import (
    LiveServerUnavailable,
    is_live_server_configured,
    missing_live_server_settings,
    replace_document_content,
)

DOCUMENT_ID = uuid.uuid4()


@pytest.fixture
def live_settings(settings):
    settings.LIVE_URL = "http://live.test/live/"
    settings.LIVE_SERVER_SECRET_KEY = "secret"
    return settings


def build_response(status_code=200, payload=None):
    response = mock.Mock()
    response.status_code = status_code
    response.json.return_value = payload
    return response


@pytest.mark.unit
class TestReplaceDocumentContent:
    def test_sends_the_stored_snapshot_and_returns_the_rewritten_one(self, live_settings):
        payload = {
            "description_binary": base64.b64encode(b"new-binary").decode(),
            "description_html": "<p>new</p>",
            "description_json": {"type": "doc"},
        }

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            document = replace_document_content(
                document_id=DOCUMENT_ID,
                description_html="<p>new</p>",
                name="Page name",
                description_binary=b"old-binary",
            )

        assert document == {
            "description_binary": b"new-binary",
            "description_html": "<p>new</p>",
            "description_json": {"type": "doc"},
        }

        url, kwargs = post.call_args[0][0], post.call_args[1]
        assert url == f"http://live.test/live/document/{DOCUMENT_ID}/content"
        assert kwargs["headers"] == {"live-server-secret-key": "secret"}
        assert kwargs["json"] == {
            "description_html": "<p>new</p>",
            "name": "Page name",
            "description_binary": base64.b64encode(b"old-binary").decode(),
        }

    def test_accepts_a_memoryview_snapshot(self, live_settings):
        # psycopg hands binary columns back as memoryview
        payload = {
            "description_binary": base64.b64encode(b"new-binary").decode(),
            "description_html": "<p>new</p>",
            "description_json": {},
        }

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            replace_document_content(
                document_id=DOCUMENT_ID,
                description_html="<p>new</p>",
                name="Page name",
                description_binary=memoryview(b"old-binary"),
            )

        assert post.call_args[1]["json"]["description_binary"] == base64.b64encode(b"old-binary").decode()

    def test_sends_no_snapshot_when_the_page_has_none(self, live_settings):
        payload = {
            "description_binary": base64.b64encode(b"new-binary").decode(),
            "description_html": "<p>new</p>",
            "description_json": {},
        }

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            replace_document_content(
                document_id=DOCUMENT_ID,
                description_html="<p>new</p>",
                name="Page name",
                description_binary=None,
            )

        assert post.call_args[1]["json"]["description_binary"] is None

    def test_only_sends_the_fields_that_changed(self, live_settings):
        # rewriting the title with the stored name would revert a rename made in the editor,
        # and rewriting the body with the stored HTML would revert unsaved edits
        payload = {
            "description_binary": base64.b64encode(b"new-binary").decode(),
            "description_html": "<p>new</p>",
            "description_json": {},
        }

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            replace_document_content(DOCUMENT_ID, description_html="<p>new</p>", description_binary=b"old-binary")
        assert "name" not in post.call_args[1]["json"]

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            replace_document_content(DOCUMENT_ID, name="New name", description_binary=b"old-binary")
        assert "description_html" not in post.call_args[1]["json"]
        assert post.call_args[1]["json"]["name"] == "New name"

    def test_sends_an_empty_paragraph_for_empty_html(self, live_settings):
        payload = {
            "description_binary": base64.b64encode(b"new-binary").decode(),
            "description_html": "<p></p>",
            "description_json": {},
        }

        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(payload=payload)) as post:
            replace_document_content(DOCUMENT_ID, description_html="", description_binary=b"old-binary")

        assert post.call_args[1]["json"]["description_html"] == "<p></p>"

    def test_raises_when_the_live_server_is_not_configured(self, settings):
        settings.LIVE_URL = None
        settings.LIVE_SERVER_SECRET_KEY = "secret"

        assert is_live_server_configured() is False
        assert missing_live_server_settings() == ["LIVE_BASE_URL"]

        with mock.patch("plane.utils.live_server.requests.post") as post:
            with pytest.raises(LiveServerUnavailable, match="LIVE_BASE_URL"):
                replace_document_content(DOCUMENT_ID, "<p>new</p>", "Page name")

        post.assert_not_called()

    def test_raises_without_a_secret_key(self, settings):
        settings.LIVE_URL = "http://live.test/live/"
        settings.LIVE_SERVER_SECRET_KEY = None

        assert missing_live_server_settings() == ["LIVE_SERVER_SECRET_KEY"]

        with mock.patch("plane.utils.live_server.requests.post") as post:
            with pytest.raises(LiveServerUnavailable, match="LIVE_SERVER_SECRET_KEY"):
                replace_document_content(DOCUMENT_ID, "<p>new</p>", "Page name")

        post.assert_not_called()

    def test_raises_on_an_error_response(self, live_settings):
        with mock.patch("plane.utils.live_server.requests.post", return_value=build_response(status_code=401)):
            with pytest.raises(LiveServerUnavailable, match="401"):
                replace_document_content(DOCUMENT_ID, "<p>new</p>", "Page name")

    def test_raises_when_the_live_server_is_unreachable(self, live_settings):
        with mock.patch("plane.utils.live_server.requests.post", side_effect=requests.ConnectionError()):
            with pytest.raises(LiveServerUnavailable, match="could not be reached"):
                replace_document_content(DOCUMENT_ID, "<p>new</p>", "Page name")

    def test_raises_on_an_unexpected_payload(self, live_settings):
        with mock.patch(
            "plane.utils.live_server.requests.post",
            return_value=build_response(payload={"description_html": "<p>new</p>"}),
        ):
            with pytest.raises(LiveServerUnavailable, match="malformed"):
                replace_document_content(DOCUMENT_ID, "<p>new</p>", "Page name")

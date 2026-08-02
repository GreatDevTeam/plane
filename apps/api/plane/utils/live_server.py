# Copyright (c) 2023-present Plane Software, Inc. and contributors
# SPDX-License-Identifier: AGPL-3.0-only
# See the LICENSE file for details.

# Python imports
import base64

# Third party imports
import requests

# Django imports
from django.conf import settings

# Module imports
from plane.utils.exception_logger import log_exception
from plane.utils.url import normalize_url_path

# The call happens inside an API request, so it must not hang for long
LIVE_SERVER_TIMEOUT = 15


class LiveServerUnavailable(Exception):
    """The live server could not rewrite a collaboratively edited document.

    Raised instead of falling back to rebuilding the snapshot from HTML: a rebuilt snapshot is a Yjs
    document unrelated to the one the clients hold, and Yjs merges documents instead of replacing them,
    so every browser that cached the page ends up showing the old and the new content at once - and
    pushes that merged state, title included, back to the server.
    """


def is_live_server_configured():
    """Whether the API knows how to reach the live server."""
    return bool(settings.LIVE_URL and settings.LIVE_SERVER_SECRET_KEY)


def missing_live_server_settings():
    """The settings that have to be set for the API to reach the live server, and are not."""
    missing = []
    if not settings.LIVE_URL:
        missing.append("LIVE_BASE_URL")
    if not settings.LIVE_SERVER_SECRET_KEY:
        missing.append("LIVE_SERVER_SECRET_KEY")
    return missing


def replace_document_content(document_id, description_html=None, name=None, description_binary=None):
    """Ask the live server to rewrite the content of a collaboratively edited document.

    The editor never renders `description_html`: it renders the Yjs snapshot kept in
    `description_binary`, and the document title is part of that snapshot too. The live server rewrites
    the content inside the existing snapshot - and inside its in-memory copy when the page is open right
    now - so the result is an update of the document the clients hold instead of an unrelated one, and
    open editors see the change instead of storing their stale state back over it.

    Only the fields that are passed are rewritten, so updating the body does not revert a title that was
    changed in the editor in the meantime, and the other way around.

    Returns the document formats to persist. Raises `LiveServerUnavailable` when the live server is not
    configured, unreachable or fails - the page must then be left untouched rather than desynced.
    """
    if not is_live_server_configured():
        raise LiveServerUnavailable(
            "The live server is not configured: set " + " and ".join(missing_live_server_settings())
        )

    url = normalize_url_path(f"{settings.LIVE_URL}/document/{document_id}/content")

    payload = {
        "description_binary": (base64.b64encode(bytes(description_binary)).decode() if description_binary else None)
    }
    if description_html is not None:
        payload["description_html"] = description_html or "<p></p>"
    if name is not None:
        payload["name"] = name

    try:
        response = requests.post(
            url,
            json=payload,
            headers={"live-server-secret-key": settings.LIVE_SERVER_SECRET_KEY},
            timeout=LIVE_SERVER_TIMEOUT,
        )
    except requests.RequestException as e:
        log_exception(e)
        raise LiveServerUnavailable(f"The live server at {url} could not be reached: {e}") from e

    if response.status_code != 200:
        error = LiveServerUnavailable(
            f"The live server at {url} returned {response.status_code} for document {document_id}"
        )
        log_exception(error, warning=True)
        raise error

    try:
        data = response.json()
        return {
            "description_binary": base64.b64decode(data["description_binary"]),
            "description_html": data["description_html"],
            "description_json": data["description_json"],
        }
    except (ValueError, KeyError, TypeError) as e:
        log_exception(e)
        raise LiveServerUnavailable(f"The live server at {url} returned a malformed response: {e}") from e

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


def replace_document_content(document_id, description_html, name, description_binary=None):
    """Ask the live server to rewrite the content of a collaboratively edited document.

    The editor never renders `description_html`: it renders the Yjs snapshot kept in
    `description_binary`, and the document title is part of that snapshot too. Rebuilding the snapshot
    from HTML here would produce a document unrelated to the one the clients hold, and Yjs merges
    documents instead of replacing them - browsers that cached the page would end up showing the old
    and the new content at once. The live server therefore rewrites the content inside the existing
    snapshot and pushes the resulting update to every server that currently has the document open, so
    editors that are open right now see the change instead of storing their stale state back over it.

    Returns the snapshot to persist, or None when the live server is not configured or unreachable -
    the caller is then expected to fall back to dropping the snapshot.
    """
    live_url = settings.LIVE_URL
    secret_key = settings.LIVE_SERVER_SECRET_KEY

    if not live_url or not secret_key:
        return None

    url = normalize_url_path(f"{live_url}/document/{document_id}/content")

    payload = {
        "description_html": description_html or "<p></p>",
        "name": name or "",
        "description_binary": (base64.b64encode(bytes(description_binary)).decode() if description_binary else None),
    }

    try:
        response = requests.post(
            url,
            json=payload,
            headers={"live-server-secret-key": secret_key},
            timeout=LIVE_SERVER_TIMEOUT,
        )
        if response.status_code != 200:
            log_exception(
                Exception(f"Live server returned {response.status_code} while updating document {document_id}"),
                warning=True,
            )
            return None

        data = response.json()
        return {
            "description_binary": base64.b64decode(data["description_binary"]),
            "description_html": data["description_html"],
            "description_json": data["description_json"],
        }
    except (requests.RequestException, ValueError, KeyError, TypeError) as e:
        log_exception(e)
        return None

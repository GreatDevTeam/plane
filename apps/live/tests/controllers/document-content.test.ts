/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { describe, expect, it } from "vitest";
import * as Y from "yjs";
import {
  getAllDocumentFormatsFromDocumentEditorBinaryData,
  getBinaryDataFromDocumentEditorHTMLString,
  replaceDocumentEditorBinaryDataContent,
  replaceDocumentEditorContent,
} from "@plane/editor";

const read = (state: Uint8Array) => {
  const { contentHTML, titleHTML } = getAllDocumentFormatsFromDocumentEditorBinaryData(state, true);
  return { html: contentHTML, title: titleHTML };
};

/** the text of each block in the body, so that the assertions do not depend on the editor markup */
const text = (html: string) =>
  html
    .split(/<[^>]+>/)
    .map((part) => part.trim())
    .filter(Boolean)
    .join("|");

describe("replacing the content of a collaboratively edited document", () => {
  it("replaces the body and the title of the document instead of appending to them", () => {
    const stored = getBinaryDataFromDocumentEditorHTMLString("<p>old content</p>", "Old title");

    const { encodedDocument } = replaceDocumentEditorBinaryDataContent({
      existingBinaryData: stored,
      descriptionHTML: "<p>new content</p>",
      title: "New title",
    });

    expect(text(read(encodedDocument).html)).toBe("new content");
    expect(read(encodedDocument).title).toBe("New title");
  });

  it("makes a client that cached the previous version converge on the new content", () => {
    // the browser keeps the Yjs document in IndexedDB, and Yjs sync is a merge and never a replace:
    // a rewrite has to carry the deletion of the old content, or the client shows both versions
    const stored = getBinaryDataFromDocumentEditorHTMLString("<p>old content</p>", "Old title");
    const cachedClient = new Y.Doc();
    Y.applyUpdate(cachedClient, stored);

    const { incrementalUpdate } = replaceDocumentEditorBinaryDataContent({
      existingBinaryData: stored,
      descriptionHTML: "<p>new content</p>",
      title: "New title",
    });
    Y.applyUpdate(cachedClient, incrementalUpdate);

    const client = read(Y.encodeStateAsUpdate(cachedClient));
    expect(text(client.html)).toBe("new content");
    expect(client.title).toBe("New title");
  });

  it("leaves the title alone when only the body is rewritten, and the other way around", () => {
    const stored = getBinaryDataFromDocumentEditorHTMLString("<p>old content</p>", "Old title");

    const bodyOnly = replaceDocumentEditorBinaryDataContent({
      existingBinaryData: stored,
      descriptionHTML: "<p>new content</p>",
    });
    expect(read(bodyOnly.encodedDocument).title).toBe("Old title");
    expect(text(read(bodyOnly.encodedDocument).html)).toBe("new content");

    const titleOnly = replaceDocumentEditorBinaryDataContent({
      existingBinaryData: stored,
      title: "New title",
    });
    expect(read(titleOnly.encodedDocument).title).toBe("New title");
    expect(text(read(titleOnly.encodedDocument).html)).toBe("old content");
  });

  it("drops the content the loaded document has on top of the stored snapshot", () => {
    // this is what the controller does when the live server holds the page: hocuspocus stores the
    // document on a debounce, so its in-memory copy is ahead of the snapshot the API sends along.
    // Rewriting the stale snapshot would only delete the content that snapshot knows about.
    const stored = getBinaryDataFromDocumentEditorHTMLString("<p>old content</p>", "Old title");
    const loaded = new Y.Doc();
    Y.applyUpdate(loaded, stored);
    loaded.transact(() => {
      const fragment = loaded.getXmlFragment("default");
      const paragraph = new Y.XmlElement("paragraph");
      paragraph.insert(0, [new Y.XmlText("not stored yet")]);
      fragment.insert(fragment.length, [paragraph]);
    });

    // what the previous implementation did: rewrite the snapshot and hand the update over
    const staleRewrite = new Y.Doc();
    Y.applyUpdate(staleRewrite, Y.encodeStateAsUpdate(loaded));
    Y.applyUpdate(
      staleRewrite,
      replaceDocumentEditorBinaryDataContent({
        existingBinaryData: stored,
        descriptionHTML: "<p>new content</p>",
      }).incrementalUpdate
    );
    expect(text(read(Y.encodeStateAsUpdate(staleRewrite)).html)).toBe("new content|not stored yet");

    // rewriting the loaded document itself deletes everything it actually holds
    replaceDocumentEditorContent({ yDoc: loaded, descriptionHTML: "<p>new content</p>" });
    expect(text(read(Y.encodeStateAsUpdate(loaded)).html)).toBe("new content");
  });

  it("rewrites in a single transaction and carries no origin, so hocuspocus does not try to store it", () => {
    // hocuspocus runs its store hooks for every update that carries a transaction origin, and they write to the
    // API as the user the origin connection belongs to — an update coming from the REST API has no such user
    const document = new Y.Doc();
    Y.applyUpdate(document, getBinaryDataFromDocumentEditorHTMLString("<p>old content</p>", "Old title"));

    const origins: unknown[] = [];
    document.on("update", (_update: Uint8Array, origin: unknown) => origins.push(origin));

    replaceDocumentEditorContent({ yDoc: document, descriptionHTML: "<p>new content</p>", title: "New title" });

    expect(origins).toEqual([null]);
  });
});

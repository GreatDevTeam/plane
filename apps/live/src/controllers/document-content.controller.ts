/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { Hocuspocus } from "@hocuspocus/server";
import type { Request, Response } from "express";
import * as Y from "yjs";
import { z } from "zod";
// plane imports
import { Controller, Middleware, Post } from "@plane/decorators";
import {
  convertBase64StringToBinaryData,
  convertBinaryDataToBase64String,
  getAllDocumentFormatsFromDocumentEditorBinaryData,
  replaceDocumentEditorBinaryDataContent,
  replaceDocumentEditorContent,
} from "@plane/editor";
import { logger } from "@plane/logger";
// extensions
import { applyUpdateAcrossServers } from "@/extensions/document-update-handler";
// lib
import { requireSecretKey } from "@/lib/auth-middleware";

const replaceContentSchema = z
  .object({
    description_html: z.string().optional(),
    name: z.string().optional(),
    description_binary: z.string().nullish(),
  })
  .refine((data) => data.description_html !== undefined || data.name !== undefined, {
    message: "Either description_html or name has to be provided",
  });

@Controller("/document")
export class DocumentContentController {
  private readonly hocusPocusServer: Hocuspocus;

  constructor(hocusPocusServer: Hocuspocus) {
    this.hocusPocusServer = hocusPocusServer;
  }

  /**
   * Rewrite the content of a document that is edited collaboratively, from outside the editor.
   *
   * The content is replaced inside the Yjs document the clients already hold, so that the result is an update of it
   * rather than an unrelated document — Yjs sync is a merge and never a replace, so a fresh document would leave
   * every client that cached the page showing the old and the new content at once.
   *
   * When this server has the document open, its in-memory copy is the only up to date one (hocuspocus stores it to
   * the database on a debounce) so the rewrite is applied to it directly: that broadcasts the change to the
   * connected editors, propagates it to the other servers through the hocuspocus Redis extension and makes it part
   * of the state that gets stored, instead of being overwritten by it. Otherwise the snapshot sent by the caller is
   * used as the base and the resulting update is published to whichever server holds the document.
   *
   * Only the fields that are present in the request are rewritten, so an update of the body does not revert a title
   * that was changed in the editor in the meantime, and vice versa.
   */
  @Post("/:documentName/content")
  @Middleware(requireSecretKey)
  async replaceContent(req: Request, res: Response) {
    const { documentName } = req.params;

    try {
      const { description_html, name, description_binary } = replaceContentSchema.parse(req.body);

      const loadedDocument = this.hocusPocusServer.documents.get(documentName);
      const { encodedDocument, incrementalUpdate } = loadedDocument
        ? this.rewriteLoadedDocument(loadedDocument, description_html, name)
        : replaceDocumentEditorBinaryDataContent({
            existingBinaryData: description_binary
              ? new Uint8Array(convertBase64StringToBinaryData(description_binary))
              : new Uint8Array(),
            descriptionHTML: description_html,
            title: name,
          });

      const { contentBinaryEncoded, contentHTML, contentJSON } = getAllDocumentFormatsFromDocumentEditorBinaryData(
        encodedDocument,
        false
      );

      await applyUpdateAcrossServers(
        this.hocusPocusServer,
        documentName,
        convertBinaryDataToBase64String(incrementalUpdate)
      );

      return res.status(200).json({
        description_binary: contentBinaryEncoded,
        description_html: contentHTML,
        description_json: contentJSON,
      });
    } catch (error) {
      if (error instanceof z.ZodError) {
        const validationErrors = error.errors.map((err) => ({
          path: err.path.join("."),
          message: err.message,
        }));
        logger.error("DOCUMENT_CONTENT_CONTROLLER: Validation error", { validationErrors });
        return res.status(400).json({
          message: "Validation error",
          context: { validationErrors },
        });
      }

      logger.error(`DOCUMENT_CONTENT_CONTROLLER: Failed to replace the content of ${documentName}`, error);
      return res.status(500).json({
        message: "Internal server error.",
      });
    }
  }

  /**
   * Rewrite the content of a document this server currently holds in memory.
   *
   * Rewriting the caller's snapshot instead would only delete the content that snapshot knows about: anything the
   * editors changed since it was stored would survive the rewrite and end up appended to the new content.
   *
   * The rewrite carries no transaction origin, for the reason documented on `applyUpdateToLoadedDocument`.
   */
  private rewriteLoadedDocument(document: Y.Doc, descriptionHTML: string | undefined, title: string | undefined) {
    const stateVector = Y.encodeStateVector(document);

    replaceDocumentEditorContent({ yDoc: document, descriptionHTML, title });

    return {
      encodedDocument: Y.encodeStateAsUpdate(document),
      incrementalUpdate: Y.encodeStateAsUpdate(document, stateVector),
    };
  }
}

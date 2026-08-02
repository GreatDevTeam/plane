/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { Hocuspocus } from "@hocuspocus/server";
import type { Request, Response } from "express";
import { z } from "zod";
// plane imports
import { Controller, Middleware, Post } from "@plane/decorators";
import {
  convertBase64StringToBinaryData,
  convertBinaryDataToBase64String,
  getAllDocumentFormatsFromDocumentEditorBinaryData,
  replaceDocumentEditorBinaryDataContent,
} from "@plane/editor";
import { logger } from "@plane/logger";
// extensions
import { applyUpdateAcrossServers } from "@/extensions/document-update-handler";
// lib
import { requireSecretKey } from "@/lib/auth-middleware";

const replaceContentSchema = z.object({
  description_html: z.string(),
  name: z.string().optional(),
  description_binary: z.string().nullish(),
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
   * The caller sends the snapshot it has stored along with the new HTML and title; the content is replaced inside
   * that same Yjs document so the result is an update of it rather than an unrelated document. The response is the
   * snapshot the caller has to persist, and the update is applied to every server that currently holds the
   * document, so open editors see the change and do not store their stale state back over it.
   */
  @Post("/:documentName/content")
  @Middleware(requireSecretKey)
  async replaceContent(req: Request, res: Response) {
    const { documentName } = req.params;

    try {
      const { description_html, name, description_binary } = replaceContentSchema.parse(req.body);

      const existingBinaryData = description_binary
        ? new Uint8Array(convertBase64StringToBinaryData(description_binary))
        : new Uint8Array();

      const { encodedDocument, incrementalUpdate } = replaceDocumentEditorBinaryDataContent({
        existingBinaryData,
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
}

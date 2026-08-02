/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { Extension, Hocuspocus, onConfigurePayload } from "@hocuspocus/server";
import * as Y from "yjs";
// plane imports
import { convertBase64StringToBinaryData } from "@plane/editor";
import { logger } from "@plane/logger";
import { Redis } from "@/extensions/redis";
import { AdminCommand, isApplyDocumentUpdateCommand } from "@/types/admin-commands";
import type { ApplyDocumentUpdateCommandData } from "@/types/admin-commands";

/** Transaction origin used for updates that did not come from a client connection */
export const EXTERNAL_UPDATE_ORIGIN = "external-document-update";

/**
 * Apply a Yjs update to a document that is loaded in this server's memory.
 *
 * Applying the update to the live document is what makes a change performed outside the editor visible: it is
 * broadcast to every connected client and it becomes part of the state hocuspocus stores back to the database,
 * which would otherwise overwrite the change with the stale in-memory document.
 *
 * @returns whether the document was loaded on this server
 */
export const applyUpdateToLoadedDocument = (instance: Hocuspocus, docId: string, update: string): boolean => {
  const document = instance.documents.get(docId);
  if (!document) return false;

  Y.applyUpdate(document, convertBase64StringToBinaryData(update), EXTERNAL_UPDATE_ORIGIN);
  return true;
};

/**
 * Extension to apply document updates published by other servers (or by the REST API) via the Redis admin channel.
 *
 * Yjs updates are idempotent, so it is safe for every server to apply the same update.
 */
export class DocumentUpdateHandler implements Extension {
  name = "DocumentUpdateHandler";
  priority = 999;

  async onConfigure({ instance }: onConfigurePayload) {
    const redisExt = instance.configuration.extensions.find((ext) => ext instanceof Redis);

    if (!redisExt) {
      logger.warn("[DOCUMENT_UPDATE_HANDLER] Redis extension not found");
      return;
    }

    redisExt.onAdminCommand<ApplyDocumentUpdateCommandData>(AdminCommand.APPLY_DOCUMENT_UPDATE, async (data) => {
      if (!isApplyDocumentUpdateCommand(data)) {
        logger.error("[DOCUMENT_UPDATE_HANDLER] Received invalid apply document update command");
        return;
      }

      try {
        const applied = applyUpdateToLoadedDocument(instance, data.docId, data.update);
        logger.info(
          applied
            ? `[DOCUMENT_UPDATE_HANDLER] Applied external update to ${data.docId}`
            : `[DOCUMENT_UPDATE_HANDLER] Document ${data.docId} not loaded here, nothing to apply`
        );
      } catch (error) {
        logger.error(`[DOCUMENT_UPDATE_HANDLER] Failed to apply external update to ${data.docId}:`, error);
      }
    });

    logger.info("[DOCUMENT_UPDATE_HANDLER] Registered with Redis extension");
  }
}

/**
 * Apply a Yjs update to a document on every server that currently holds it in memory.
 *
 * The update is published on the admin channel, which this server is subscribed to as well, so the document is
 * updated here too. Servers that do not have the document loaded ignore the command — they read the already
 * updated state from the database the next time the document is opened.
 */
export const applyUpdateAcrossServers = async (instance: Hocuspocus, docId: string, update: string): Promise<void> => {
  const redisExt = instance.configuration.extensions.find((ext) => ext instanceof Redis);

  if (!redisExt) {
    logger.warn("[DOCUMENT_UPDATE] Redis extension not found, applying the update locally only");
    applyUpdateToLoadedDocument(instance, docId, update);
    return;
  }

  const commandData: ApplyDocumentUpdateCommandData = {
    command: AdminCommand.APPLY_DOCUMENT_UPDATE,
    docId,
    update,
    originServer: instance.configuration.name || "unknown",
    timestamp: new Date().toISOString(),
  };

  const receivers = await redisExt.publishAdminCommand(commandData);
  logger.info(`[DOCUMENT_UPDATE] Published update for ${docId} to ${receivers} server(s)`);
};

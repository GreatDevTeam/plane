/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TIssuePropertyRelationType, TIssuePropertyType } from "@plane/types";

/** The field types the settings screen offers, in the order the dropdown lists them. */
export const ISSUE_PROPERTY_TYPES: TIssuePropertyType[] = [
  "TEXT",
  "DECIMAL",
  "BOOLEAN",
  "DATETIME",
  "OPTION",
  "RELATION",
  "URL",
  "EMAIL",
  "FILE",
];

export const ISSUE_PROPERTY_RELATION_TYPES: TIssuePropertyRelationType[] = ["USER", "ISSUE"];

/** `min`/`max` only bound a number, `max_length` only bounds the text backed types. */
export const propertyTakesLengthBound = (propertyType: TIssuePropertyType) =>
  propertyType === "TEXT" || propertyType === "URL" || propertyType === "EMAIL";

export const propertyTakesRangeBound = (propertyType: TIssuePropertyType) => propertyType === "DECIMAL";

/** Only a field whose value is an id out of a set can sensibly hold more than one. */
export const propertyTakesMultipleValues = (propertyType: TIssuePropertyType) =>
  propertyType === "OPTION" || propertyType === "RELATION";

/**
 * The api key a display name defaults to. `name` is what the export column, the webhook
 * payload and the public API address the field by, so it has to stay a stable ascii
 * identifier while the display name is free text.
 */
export const toPropertyKey = (displayName: string) =>
  displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 255);

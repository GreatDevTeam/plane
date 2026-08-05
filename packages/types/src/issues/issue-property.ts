/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TLogoProps } from "../common";

export type TIssuePropertyType =
  | "TEXT"
  | "DECIMAL"
  | "OPTION"
  | "BOOLEAN"
  | "DATETIME"
  | "RELATION"
  | "URL"
  | "EMAIL"
  | "FILE";

/** What a `RELATION` property points at. */
export type TIssuePropertyRelationType = "USER" | "ISSUE";

/** Bounds the API enforces when a value is submitted. */
export type TIssuePropertySettings = {
  min?: number;
  max?: number;
  max_length?: number;
};

/** A user defined field on a work item type. */
export type TIssueProperty = {
  id: string;
  /** The stable api key — `display_name` is what the UI renders */
  name: string;
  display_name: string;
  description: string;
  property_type: TIssuePropertyType;
  relation_type: TIssuePropertyRelationType | null;
  is_required: boolean;
  is_active: boolean;
  is_multi: boolean;
  default_value: TIssuePropertyValue[];
  settings: TIssuePropertySettings;
  sort_order: number;
  logo_props: TLogoProps;
  issue_type_id: string;
  workspace_id: string;
  external_source: string | null;
  external_id: string | null;
  created_at: string;
  updated_at: string;
  created_by: string | null;
  updated_by: string | null;
};

/** One selectable choice of an `OPTION` property. */
export type TIssuePropertyOption = {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  is_default: boolean;
  sort_order: number;
  logo_props: TLogoProps;
  parent_id: string | null;
  property_id: string;
  workspace_id: string;
  external_source: string | null;
  external_id: string | null;
  created_at: string;
  updated_at: string;
  created_by: string | null;
  updated_by: string | null;
};

/**
 * One stored value. `DECIMAL` comes back as a number and `BOOLEAN` as a boolean;
 * every other type — including the option, member and work item ids — as a string.
 */
export type TIssuePropertyValue = string | number | boolean;

/**
 * The values of one work item, keyed by property id. Every property of the work
 * item's type is present, so an empty list means "not set" rather than "not loaded".
 */
export type TIssuePropertyValues = Record<string, TIssuePropertyValue[]>;

/**
 * The values of a page of work items, keyed by work item id and then by property id.
 * A work item only carries the properties of its own type.
 */
export type TBulkIssuePropertyValues = Record<string, TIssuePropertyValues>;

/**
 * How a custom property is keyed among the display properties of a layout, so that its
 * visibility on the card and its spreadsheet column can be toggled like a built-in one.
 */
export type TIssuePropertyDisplayKey = `property_${string}`;

/**
 * How a custom property is keyed among the filter properties of a layout. It is the same
 * key as the display one — the backend reads `property_<uuid>__<operator>` off the filter
 * expression and rewrites it onto the property's own value column.
 */
export type TIssuePropertyFilterKey = TIssuePropertyDisplayKey;

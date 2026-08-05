/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TFilterValue } from "../expression";
import type { TBaseFilterFieldConfig } from "./shared";

/**
 * Extended filter types
 *
 * The editors a user defined work item property needs and the core ones do not cover.
 * Everything else maps onto a core field type: an option or member property onto a
 * multi select, a boolean onto a single select, a datetime onto the date pickers.
 */
export const EXTENDED_FILTER_FIELD_TYPE = {
  TEXT: "text",
  NUMBER: "number",
} as const;

/**
 * Free text filter configuration - a single line input.
 * - defaultValue: Initial text
 * - placeholder: Shown while the input is empty
 */
export type TTextFilterFieldConfig<V extends TFilterValue> = TBaseFilterFieldConfig & {
  type: typeof EXTENDED_FILTER_FIELD_TYPE.TEXT;
  defaultValue?: V;
  placeholder?: string;
};

/**
 * Number filter configuration - a numeric input.
 * - defaultValue: Initial number
 * - min: Smallest accepted value
 * - max: Largest accepted value
 */
export type TNumberFilterFieldConfig<V extends TFilterValue> = TBaseFilterFieldConfig & {
  type: typeof EXTENDED_FILTER_FIELD_TYPE.NUMBER;
  defaultValue?: V;
  min?: number;
  max?: number;
};

// -------- UNION TYPES --------

/**
 * All extended filter configurations
 */
export type TExtendedFilterFieldConfigs<V extends TFilterValue = TFilterValue> =
  | TTextFilterFieldConfig<V>
  | TNumberFilterFieldConfig<V>;

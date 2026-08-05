/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// plane imports
import type { TBaseFilterFieldConfig } from "@plane/types";
import { FILTER_FIELD_TYPE } from "@plane/types";
// local imports
import { createFilterFieldConfig } from "./shared";

// ------------ Free text filter ------------

/**
 * Free text filter configuration
 */
export type TTextConfig = TBaseFilterFieldConfig & {
  defaultValue?: string;
  placeholder?: string;
};

/**
 * Helper to get the free text input config
 * @param config - Text-specific configuration
 * @returns The text input config
 */
export const getTextInputConfig = (config: TTextConfig) =>
  createFilterFieldConfig<typeof FILTER_FIELD_TYPE.TEXT, string>({
    type: FILTER_FIELD_TYPE.TEXT,
    ...config,
  });

// ------------ Number filter ------------

/**
 * Number filter configuration
 */
export type TNumberConfig = TBaseFilterFieldConfig & {
  defaultValue?: number;
  min?: number;
  max?: number;
};

/**
 * Helper to get the number input config
 * @param config - Number-specific configuration
 * @returns The number input config
 */
export const getNumberInputConfig = (config: TNumberConfig) =>
  createFilterFieldConfig<typeof FILTER_FIELD_TYPE.NUMBER, number>({
    type: FILTER_FIELD_TYPE.NUMBER,
    ...config,
  });

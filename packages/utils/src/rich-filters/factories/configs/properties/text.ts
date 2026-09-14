/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// plane imports
import type { TFilterProperty } from "@plane/types";
import { EXTENDED_EQUALITY_OPERATOR } from "@plane/types";
// local imports
import { getTextInputConfig } from "../extended";
import type { TCreateFilterConfig } from "../shared";
import { createFilterConfig, createOperatorConfigEntry } from "../shared";
import type { TCustomPropertyFilterParams } from "./shared";

/**
 * Text property filter specific params
 */
export type TCreateTextPropertyFilterParams = TCustomPropertyFilterParams<undefined> & {
  placeholder?: string;
};

/**
 * Get the text property filter config.
 *
 * Covers the url and email properties too — all three store their value in the same
 * column, and all three are searched the same way.
 * @param key - The filter key to use
 * @returns A function that takes parameters and returns the text property filter config
 */
export const getTextPropertyFilterConfig =
  <P extends TFilterProperty>(key: P): TCreateFilterConfig<P, TCreateTextPropertyFilterParams> =>
  (params: TCreateTextPropertyFilterParams) =>
    createFilterConfig({
      id: key,
      ...params,
      label: params.propertyDisplayName,
      icon: params.filterIcon,
      supportedOperatorConfigsMap: new Map([
        createOperatorConfigEntry(EXTENDED_EQUALITY_OPERATOR.CONTAINS, params, (updatedParams) =>
          getTextInputConfig(updatedParams)
        ),
      ]),
    });

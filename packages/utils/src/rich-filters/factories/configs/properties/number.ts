/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// plane imports
import type { TFilterProperty } from "@plane/types";
import { EQUALITY_OPERATOR } from "@plane/types";
// local imports
import { getNumberInputConfig } from "../extended";
import type { TCreateFilterConfig } from "../shared";
import { createFilterConfig, createOperatorConfigEntry } from "../shared";
import type { TCustomPropertyFilterParams } from "./shared";

/**
 * Number property filter specific params
 */
export type TCreateNumberPropertyFilterParams = TCustomPropertyFilterParams<undefined> & {
  min?: number;
  max?: number;
};

/**
 * Get the number property filter config
 * @param key - The filter key to use
 * @returns A function that takes parameters and returns the number property filter config
 */
export const getNumberPropertyFilterConfig =
  <P extends TFilterProperty>(key: P): TCreateFilterConfig<P, TCreateNumberPropertyFilterParams> =>
  (params: TCreateNumberPropertyFilterParams) =>
    createFilterConfig({
      id: key,
      ...params,
      label: params.propertyDisplayName,
      icon: params.filterIcon,
      supportedOperatorConfigsMap: new Map([
        createOperatorConfigEntry(EQUALITY_OPERATOR.EXACT, params, (updatedParams) =>
          getNumberInputConfig(updatedParams)
        ),
      ]),
    });

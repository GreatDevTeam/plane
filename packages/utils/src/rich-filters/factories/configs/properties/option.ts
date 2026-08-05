/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// plane imports
import type { TFilterProperty, TIssuePropertyOption } from "@plane/types";
import { COLLECTION_OPERATOR, EQUALITY_OPERATOR } from "@plane/types";
// local imports
import { getMultiSelectConfig } from "../core";
import type { TCreateFilterConfig } from "../shared";
import { createFilterConfig, createOperatorConfigEntry } from "../shared";
import type { TCustomPropertyFilterParams } from "./shared";

/**
 * Option property filter specific params
 */
export type TCreateOptionPropertyFilterParams = TCustomPropertyFilterParams<TIssuePropertyOption> & {
  options: TIssuePropertyOption[];
};

/**
 * Get the option property filter config — the property's own choices, picked the same
 * way a label or a state is.
 * @param key - The filter key to use
 * @returns A function that takes parameters and returns the option property filter config
 */
export const getOptionPropertyFilterConfig =
  <P extends TFilterProperty>(key: P): TCreateFilterConfig<P, TCreateOptionPropertyFilterParams> =>
  (params: TCreateOptionPropertyFilterParams) =>
    createFilterConfig({
      id: key,
      ...params,
      label: params.propertyDisplayName,
      icon: params.filterIcon,
      supportedOperatorConfigsMap: new Map([
        createOperatorConfigEntry(COLLECTION_OPERATOR.IN, params, (updatedParams) =>
          getMultiSelectConfig<TIssuePropertyOption, string, TIssuePropertyOption>(
            {
              items: updatedParams.options,
              getId: (option) => option.id,
              getLabel: (option) => option.name,
              getValue: (option) => option.id,
              getIconData: (option) => option,
            },
            { singleValueOperator: EQUALITY_OPERATOR.EXACT, ...updatedParams },
            { ...updatedParams }
          )
        ),
      ]),
    });

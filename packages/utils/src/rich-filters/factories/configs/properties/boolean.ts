/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// plane imports
import type { TFilterProperty } from "@plane/types";
import { EQUALITY_OPERATOR } from "@plane/types";
// local imports
import { getSingleSelectConfig } from "../core";
import type { TCreateFilterConfig } from "../shared";
import { createFilterConfig, createOperatorConfigEntry } from "../shared";
import type { TCustomPropertyFilterParams } from "./shared";

/** The two values a boolean property can be filtered by, as the api spells them. */
const BOOLEAN_PROPERTY_OPTIONS = [
  { id: "true", label: "Yes", value: "true" },
  { id: "false", label: "No", value: "false" },
];

/**
 * Boolean property filter specific params
 */
export type TCreateBooleanPropertyFilterParams = TCustomPropertyFilterParams<undefined>;

/**
 * Get the boolean property filter config — a Yes/No picker rather than an editor of
 * its own, since there is nothing to type.
 * @param key - The filter key to use
 * @returns A function that takes parameters and returns the boolean property filter config
 */
export const getBooleanPropertyFilterConfig =
  <P extends TFilterProperty>(key: P): TCreateFilterConfig<P, TCreateBooleanPropertyFilterParams> =>
  (params: TCreateBooleanPropertyFilterParams) =>
    createFilterConfig({
      id: key,
      ...params,
      label: params.propertyDisplayName,
      icon: params.filterIcon,
      supportedOperatorConfigsMap: new Map([
        createOperatorConfigEntry(EQUALITY_OPERATOR.EXACT, params, (updatedParams) =>
          getSingleSelectConfig<(typeof BOOLEAN_PROPERTY_OPTIONS)[number], string>(
            {
              items: BOOLEAN_PROPERTY_OPTIONS,
              getId: (option) => option.id,
              getLabel: (option) => option.label,
              getValue: (option) => option.value,
            },
            updatedParams
          )
        ),
      ]),
    });

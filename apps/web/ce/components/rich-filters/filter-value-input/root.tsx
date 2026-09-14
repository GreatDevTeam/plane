/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import React from "react";
import { observer } from "mobx-react";
// plane imports
import type {
  SingleOrArray,
  TFilterValue,
  TFilterProperty,
  TNumberFilterFieldConfig,
  TTextFilterFieldConfig,
} from "@plane/types";
import { FILTER_FIELD_TYPE } from "@plane/types";
// local imports
import type { TFilterValueInputProps } from "@/components/rich-filters/shared";
import { FilterTypedValueInput } from "./input";

/**
 * The editors core does not carry, which is what a user defined work item property is
 * filtered by: a free text match and a number. Every other property type is filtered
 * through a core editor — a select, a date picker — and never reaches here.
 */
export const AdditionalFilterValueInput = observer(function AdditionalFilterValueInput<
  P extends TFilterProperty,
  V extends TFilterValue,
>(props: TFilterValueInputProps<P, V>) {
  const { condition, filterFieldConfig, isDisabled = false, onChange } = props;
  // a condition holds one value here — neither editor writes a list
  const conditionValue = Array.isArray(condition.value) ? condition.value[0] : condition.value;
  const stringValue = conditionValue === null || conditionValue === undefined ? "" : String(conditionValue);

  if (filterFieldConfig?.type === FILTER_FIELD_TYPE.TEXT) {
    const config = filterFieldConfig as TTextFilterFieldConfig<string>;
    return (
      <FilterTypedValueInput
        type="text"
        value={stringValue}
        isDisabled={isDisabled}
        placeholder={config.placeholder}
        // an emptied input is an unfinished condition, not a match on the empty string
        onChange={(value) => onChange((value === "" ? null : value) as SingleOrArray<V>)}
      />
    );
  }

  if (filterFieldConfig?.type === FILTER_FIELD_TYPE.NUMBER) {
    const config = filterFieldConfig as TNumberFilterFieldConfig<number>;
    return (
      <FilterTypedValueInput
        type="number"
        value={stringValue}
        isDisabled={isDisabled}
        min={config.min}
        max={config.max}
        // an emptied input is an unfinished condition, not a match on the empty string
        onChange={(value) => onChange((value === "" ? null : value) as SingleOrArray<V>)}
      />
    );
  }

  return (
    // Fallback
    <div className="flex h-full cursor-not-allowed items-center px-4 text-11 text-placeholder transition-opacity duration-200">
      Filter type not supported
    </div>
  );
});

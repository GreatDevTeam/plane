/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TFilterValue } from "../expression";
import type { TNumberFilterFieldConfig, TTextFilterFieldConfig } from "../field-types";
import type { EXTENDED_EQUALITY_OPERATOR } from "../operators";

// ----------------------------- EXACT Operator -----------------------------
export type TExtendedExactOperatorConfigs = TNumberFilterFieldConfig<TFilterValue>;

// ----------------------------- IN Operator -----------------------------
export type TExtendedInOperatorConfigs = never;

// ----------------------------- RANGE Operator -----------------------------
export type TExtendedRangeOperatorConfigs = never;

// ----------------------------- CONTAINS Operator -----------------------------
export type TExtendedContainsOperatorConfigs = TTextFilterFieldConfig<TFilterValue>;

// ----------------------------- Extended Operator Specific Configs -----------------------------
export type TExtendedOperatorSpecificConfigs = {
  [EXTENDED_EQUALITY_OPERATOR.CONTAINS]: TExtendedContainsOperatorConfigs;
};

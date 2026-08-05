/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TSupportedOperators } from "@plane/types";
import { CORE_OPERATORS, EXTENDED_OPERATORS } from "@plane/types";

export type TFiltersOperatorConfigs = {
  allowedOperators: Set<TSupportedOperators>;
  allowNegative: boolean;
};

export type TUseFiltersOperatorConfigsProps = {
  workspaceSlug: string;
};

export const useFiltersOperatorConfigs = (_props: TUseFiltersOperatorConfigsProps): TFiltersOperatorConfigs => ({
  // the extended operators are what the user defined property filters are written with
  allowedOperators: new Set<TSupportedOperators>([
    ...Object.values(CORE_OPERATORS),
    ...Object.values(EXTENDED_OPERATORS),
  ]),
  allowNegative: false,
});

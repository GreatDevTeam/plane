/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useContext } from "react";
// mobx store
import { StoreContext } from "@/lib/store-context";
// plane web store
import type { IIssueTypesStore } from "@/plane-web/store/issue-types";

export const useIssueTypes = (): IIssueTypesStore => {
  const context = useContext(StoreContext);
  if (context === undefined) throw new Error("useIssueTypes must be used within StoreProvider");
  return context.issueTypes;
};

/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TIssueProperty, TIssuePropertyValue } from "@plane/types";

export type TWorkItemPropertyValueProps = {
  property: TIssueProperty;
  /** Always a list, even for a single valued property — that is how the API hands it over. */
  values: TIssuePropertyValue[];
  disabled: boolean;
  hasError: boolean;
  onChange: (values: TIssuePropertyValue[]) => void;
};

export type TWorkItemRelationPropertyValueProps = TWorkItemPropertyValueProps & {
  workspaceSlug: string;
  projectId: string;
};

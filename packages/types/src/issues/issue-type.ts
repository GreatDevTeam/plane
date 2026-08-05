/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { TLogoProps } from "../common";

export type TIssueType = {
  id: string;
  name: string;
  description: string;
  logo_props: TLogoProps;
  is_epic: boolean;
  is_default: boolean;
  is_active: boolean;
  level: number;
  workspace_id: string;
  project_ids: string[];
  external_source: string | null;
  external_id: string | null;
  created_at: string;
  updated_at: string;
  created_by: string | null;
  updated_by: string | null;
};

/**
 * Payload accepted by the work item type endpoints. `is_default`/`level` are the per
 * project enablement settings on the project scoped endpoint.
 */
export type TIssueTypePayload = Partial<
  Pick<TIssueType, "name" | "description" | "logo_props" | "is_active" | "is_default" | "level">
> & {
  issue_type_id?: string;
};

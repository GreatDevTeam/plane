/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import useSWR from "swr";
import type { TIssueServiceType } from "@plane/types";
// constants
import { WORK_ITEM_PROPERTY_VALUES, WORK_ITEM_TYPE_PROPERTIES } from "@/constants/fetch-keys";
// hooks
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
// plane web hooks
import { useIssueProperties, useIssuePropertyValues } from "@/plane-web/hooks/store";

/**
 * Loads the property definitions of a work item type and the values of one work item.
 *
 * Both are keyed through SWR, so calling this from the peek root, the browse page and
 * the sidebar itself still costs a single request each.
 */
export const useWorkItemPropertiesById = (
  workspaceSlug: string | null | undefined,
  projectId: string | null | undefined,
  workItemId: string | null | undefined,
  workItemTypeId: string | null | undefined
) => {
  const { fetchIssueTypeProperties } = useIssueProperties();
  const { fetchWorkItemPropertyValues } = useIssuePropertyValues();

  useSWR(
    workspaceSlug && workItemTypeId ? WORK_ITEM_TYPE_PROPERTIES(workspaceSlug, workItemTypeId) : null,
    workspaceSlug && workItemTypeId ? () => fetchIssueTypeProperties(workspaceSlug, workItemTypeId) : null,
    { revalidateIfStale: false, revalidateOnFocus: false }
  );

  useSWR(
    workspaceSlug && projectId && workItemId ? WORK_ITEM_PROPERTY_VALUES(workspaceSlug, projectId, workItemId) : null,
    workspaceSlug && projectId && workItemId
      ? () => fetchWorkItemPropertyValues(workspaceSlug, projectId, workItemId)
      : null,
    { revalidateIfStale: false, revalidateOnFocus: false }
  );
};

/**
 * Prefetches the custom properties of a work item, called from the peek root and the
 * browse work item page before the sidebar renders. The work item type is read from
 * the detail store, so nothing is fetched until the work item itself has loaded.
 */
export const useWorkItemProperties = (
  projectId: string | null | undefined,
  workspaceSlug: string | null | undefined,
  workItemId: string | null | undefined,
  issueServiceType: TIssueServiceType
) => {
  const {
    issue: { getIssueById },
  } = useIssueDetail(issueServiceType);
  const workItemTypeId = workItemId ? getIssueById(workItemId)?.type_id : undefined;

  useWorkItemPropertiesById(workspaceSlug, projectId, workItemId, workItemTypeId);
};

/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import useSWR from "swr";
import type { TIssueServiceType } from "@plane/types";
// constants
import {
  DRAFT_WORK_ITEM_PROPERTY_VALUES,
  WORK_ITEM_PROPERTY_VALUES,
  WORK_ITEM_TYPE_PROPERTIES,
} from "@/constants/fetch-keys";
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
 * What the create/update modal needs before it can seed its form state: the property
 * definitions of the picked work item type, and the values already saved on the work
 * item it is editing (a workspace draft keeps them in its own table).
 *
 * Both flags stay `false` until the request they stand for has landed, so the modal
 * does not seed a form off half the data and then overwrite what the user typed.
 */
export const useWorkItemModalProperties = (
  workspaceSlug: string | null | undefined,
  projectId: string | null | undefined,
  workItemId: string | null | undefined,
  workItemTypeId: string | null | undefined,
  isDraft: boolean
) => {
  const { fetchIssueTypeProperties } = useIssueProperties();
  const { fetchWorkItemPropertyValues, fetchDraftPropertyValues } = useIssuePropertyValues();

  const { data: properties } = useSWR(
    workspaceSlug && workItemTypeId ? WORK_ITEM_TYPE_PROPERTIES(workspaceSlug, workItemTypeId) : null,
    workspaceSlug && workItemTypeId ? () => fetchIssueTypeProperties(workspaceSlug, workItemTypeId) : null,
    { revalidateIfStale: false, revalidateOnFocus: false }
  );

  const canReadValues = Boolean(workspaceSlug && workItemId && (isDraft || projectId));
  const { data: savedValues } = useSWR(
    canReadValues
      ? isDraft
        ? DRAFT_WORK_ITEM_PROPERTY_VALUES(workspaceSlug!, workItemId!)
        : WORK_ITEM_PROPERTY_VALUES(workspaceSlug!, projectId!, workItemId!)
      : null,
    canReadValues
      ? () =>
          isDraft
            ? fetchDraftPropertyValues(workspaceSlug!, workItemId!)
            : fetchWorkItemPropertyValues(workspaceSlug!, projectId!, workItemId!)
      : null,
    { revalidateIfStale: false, revalidateOnFocus: false }
  );

  // only the readiness matters — what was fetched is read back off the store, which a
  // save through the modal keeps current and the SWR cache does not
  return {
    arePropertiesReady: properties !== undefined,
    // nothing to wait for when the modal is creating a work item from scratch
    areSavedValuesReady: !canReadValues || savedValues !== undefined,
  };
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

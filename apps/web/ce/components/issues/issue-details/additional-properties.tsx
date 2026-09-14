/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";
import { useWorkItemPropertiesById } from "@/plane-web/hooks/use-issue-properties";
// local imports
import { WorkItemPropertyValueRoot } from "./property-values";

export type TWorkItemAdditionalSidebarProperties = {
  workItemId: string;
  workItemTypeId: string | null;
  projectId: string;
  workspaceSlug: string;
  isEditable: boolean;
  isPeekView?: boolean;
};

/**
 * The custom properties of a work item, rendered by both the detail sidebar and the peek
 * view below the built in ones. A work item carries the properties of its own type, and a
 * property that has been deactivated keeps its values but is no longer shown.
 */
export const WorkItemAdditionalSidebarProperties = observer(function WorkItemAdditionalSidebarProperties(
  props: TWorkItemAdditionalSidebarProperties
) {
  const { workItemId, workItemTypeId, projectId, workspaceSlug, isEditable } = props;
  // store hooks
  const { getActiveIssueTypeProperties } = useIssueProperties();
  // the peek root and the browse page prefetch the same keys — SWR keeps this to one request
  useWorkItemPropertiesById(workspaceSlug, projectId, workItemId, workItemTypeId);
  // derived values
  const properties = getActiveIssueTypeProperties(workItemTypeId);

  if (properties.length === 0) return <></>;

  return (
    <>
      {properties.map((property) => (
        <WorkItemPropertyValueRoot
          key={property.id}
          property={property}
          workspaceSlug={workspaceSlug}
          projectId={projectId}
          workItemId={workItemId}
          isEditable={isEditable}
        />
      ))}
    </>
  );
});

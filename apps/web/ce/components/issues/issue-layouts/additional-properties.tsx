/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import React, { useEffect } from "react";
import { observer } from "mobx-react";
import { useParams } from "next/navigation";
import type { IIssueDisplayProperties, TIssue } from "@plane/types";
import { getEnabledWorkItemPropertyDisplayKeys, getWorkItemPropertyDisplayKey } from "@plane/utils";
// plane web components
import { WorkItemPropertyChip } from "@/plane-web/components/issues/issue-layouts/property-values";
// plane web hooks
import { useIssueProperties, useIssuePropertyValues } from "@/plane-web/hooks/store";

export type TWorkItemLayoutAdditionalProperties = {
  displayProperties: IIssueDisplayProperties;
  issue: TIssue;
};

/**
 * The custom properties of a work item on a list, board or calendar card. Only the ones
 * switched on in the display properties are rendered, and nothing is fetched at all until
 * at least one of them is — the property definitions come per work item type, the values
 * per page of work items through the bulk endpoint.
 */
export const WorkItemLayoutAdditionalProperties = observer(function WorkItemLayoutAdditionalProperties(
  props: TWorkItemLayoutAdditionalProperties
) {
  const { displayProperties, issue } = props;
  // router
  const { workspaceSlug } = useParams();
  // store hooks
  const { getActiveIssueTypeProperties, ensureIssueTypeProperties } = useIssueProperties();
  const { ensureWorkItemPropertyValues } = useIssuePropertyValues();
  // derived values
  const properties = getActiveIssueTypeProperties(issue.type_id).filter(
    (property) => !!displayProperties[getWorkItemPropertyDisplayKey(property.id)]
  );
  // a card cannot tell which of the switched on keys belong to its own type before the
  // definitions are in, so the fetch is keyed off the toggles rather than off `properties`
  const hasEnabledProperty = getEnabledWorkItemPropertyDisplayKeys(displayProperties).length > 0;

  useEffect(() => {
    if (!hasEnabledProperty) return;
    ensureIssueTypeProperties(workspaceSlug?.toString(), issue.type_id);
    ensureWorkItemPropertyValues(workspaceSlug?.toString(), issue.project_id, issue.id);
  }, [
    hasEnabledProperty,
    workspaceSlug,
    issue.type_id,
    issue.project_id,
    issue.id,
    ensureIssueTypeProperties,
    ensureWorkItemPropertyValues,
  ]);

  if (properties.length === 0) return <></>;

  return (
    <>
      {properties.map((property) => (
        <WorkItemPropertyChip key={property.id} property={property} workItemId={issue.id} />
      ))}
    </>
  );
});

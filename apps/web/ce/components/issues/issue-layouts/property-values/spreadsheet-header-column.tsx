/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { Row } from "@plane/ui";
// plane web components
import { workItemPropertyIcon } from "@/plane-web/components/issues/issue-details/property-values";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";

type TWorkItemPropertyHeaderColumnProps = {
  propertyId: string;
};

/**
 * The header of a custom property column. It carries no sort menu — ordering a work item
 * query by a custom property has no backend support yet, so there is nothing to offer.
 */
export const WorkItemPropertyHeaderColumn = observer(function WorkItemPropertyHeaderColumn(
  props: TWorkItemPropertyHeaderColumnProps
) {
  const { propertyId } = props;
  // store hooks
  const { getPropertyById } = useIssueProperties();
  // derived values
  const property = getPropertyById(propertyId);

  if (!property) return null;
  const Icon = workItemPropertyIcon(property);

  return (
    <Row className="flex w-full items-center gap-1.5 py-2 text-13 text-secondary">
      <Icon className="h-4 w-4 text-placeholder" />
      <span className="truncate">{property.display_name}</span>
    </Row>
  );
});

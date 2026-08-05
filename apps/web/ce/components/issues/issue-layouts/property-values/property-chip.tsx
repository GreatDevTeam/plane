/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { Tooltip } from "@plane/propel/tooltip";
import type { TIssueProperty } from "@plane/types";
// hooks
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web components
import { workItemPropertyIcon } from "@/plane-web/components/issues/issue-details/property-values";
// plane web hooks
import { useIssuePropertyValues } from "@/plane-web/hooks/store";
// local imports
import { useWorkItemPropertyValueLabel } from "./value-label";

type TWorkItemPropertyChipProps = {
  property: TIssueProperty;
  workItemId: string;
};

/**
 * One custom property on a list, board or calendar card — the same read only chip the
 * link and attachment counts are rendered with. A property with no value is left out
 * rather than rendered empty, as the built-in ones are.
 */
export const WorkItemPropertyChip = observer(function WorkItemPropertyChip(props: TWorkItemPropertyChipProps) {
  const { property, workItemId } = props;
  // plane hooks
  const { isMobile } = usePlatformOS();
  // store hooks
  const { getPropertyValue } = useIssuePropertyValues();
  // derived values
  const values = getPropertyValue(workItemId, property.id);
  const label = useWorkItemPropertyValueLabel(property, values);
  const Icon = workItemPropertyIcon(property);

  if (!label) return null;

  return (
    <Tooltip tooltipHeading={property.display_name} tooltipContent={label} isMobile={isMobile} renderByDefault={false}>
      <div className="flex h-5 max-w-32 flex-shrink-0 items-center gap-2 overflow-hidden rounded-sm border-[0.5px] border-strong px-2.5 py-1">
        <Icon className="h-3 w-3 flex-shrink-0" strokeWidth={2} />
        <span className="truncate text-caption-sm-regular">{label}</span>
      </div>
    </Tooltip>
  );
});

/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { useParams } from "next/navigation";
// plane imports
import type { IIssueDisplayProperties } from "@plane/types";
import { getWorkItemPropertyDisplayKey } from "@plane/utils";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";
import { useProjectWorkItemProperties } from "@/plane-web/hooks/use-issue-properties";

export type TWorkItemAdditionalDisplayProperties = {
  displayProperties: IIssueDisplayProperties;
  handleUpdate: (updatedDisplayProperties: Partial<IIssueDisplayProperties>) => void;
};

/**
 * A toggle per user defined property, rendered next to the built-in ones. Only a project
 * scoped layout has a set of properties to offer — a workspace level one spans projects
 * whose work item types differ.
 */
export const WorkItemAdditionalDisplayProperties = observer(function WorkItemAdditionalDisplayProperties(
  props: TWorkItemAdditionalDisplayProperties
) {
  const { displayProperties, handleUpdate } = props;
  // router
  const { workspaceSlug, projectId } = useParams();
  // store hooks
  const { getProjectProperties } = useIssueProperties();
  // the definitions the toggles are built from
  useProjectWorkItemProperties(workspaceSlug?.toString(), projectId?.toString());
  // derived values
  const properties = getProjectProperties(projectId?.toString());

  if (!projectId || properties.length === 0) return <></>;

  return (
    <>
      {properties.map((property) => {
        const displayKey = getWorkItemPropertyDisplayKey(property.id);
        const isEnabled = !!displayProperties[displayKey];

        return (
          <button
            key={property.id}
            type="button"
            className={`rounded-sm border px-2 py-0.5 text-11 transition-all ${
              isEnabled ? "border-accent-strong bg-accent-primary text-on-color" : "border-subtle hover:bg-layer-1"
            }`}
            onClick={() => handleUpdate({ [displayKey]: !isEnabled })}
          >
            {property.display_name}
          </button>
        );
      })}
    </>
  );
});

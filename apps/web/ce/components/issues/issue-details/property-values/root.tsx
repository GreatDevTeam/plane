/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { Info } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Tooltip } from "@plane/propel/tooltip";
import type { TIssueProperty, TIssuePropertyValue } from "@plane/types";
// components
import { SidebarPropertyListItem } from "@/components/common/layout/sidebar/property-list-item";
// hooks
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web hooks
import { useIssuePropertyValues } from "@/plane-web/hooks/store";
// local imports
import { WorkItemPropertyValueEditor, workItemPropertyIcon } from "./editor";

type TWorkItemPropertyValueRootProps = {
  property: TIssueProperty;
  workspaceSlug: string;
  projectId: string;
  workItemId: string;
  isEditable: boolean;
};

/** One custom property of a work item — its label, its editor and its error. */
export const WorkItemPropertyValueRoot = observer(function WorkItemPropertyValueRoot(
  props: TWorkItemPropertyValueRootProps
) {
  const { property, workspaceSlug, projectId, workItemId, isEditable } = props;
  // plane hooks
  const { t } = useTranslation();
  const { isMobile } = usePlatformOS();
  // store hooks
  const { getPropertyValue, getPropertyError, updatePropertyValue } = useIssuePropertyValues();
  // derived values
  const values = getPropertyValue(workItemId, property.id);
  const error = getPropertyError(workItemId, property.id);

  const handleChange = (nextValues: TIssuePropertyValue[]) => {
    // a rejection is kept on the store and rendered inline below the editor
    updatePropertyValue(workspaceSlug, projectId, workItemId, property.id, nextValues).catch(() => {});
  };

  return (
    <SidebarPropertyListItem
      icon={workItemPropertyIcon(property)}
      label={property.display_name}
      appendElement={
        <>
          {property.is_required && (
            <span className="text-danger-primary" aria-label={t("work_item_properties.required")}>
              *
            </span>
          )}
          {property.description && (
            <Tooltip tooltipContent={property.description} isMobile={isMobile}>
              <Info className="size-3 shrink-0 text-tertiary" />
            </Tooltip>
          )}
        </>
      }
      childrenClassName="flex-col items-stretch"
    >
      <WorkItemPropertyValueEditor
        property={property}
        values={values}
        disabled={!isEditable}
        hasError={Boolean(error)}
        onChange={handleChange}
        workspaceSlug={workspaceSlug}
        projectId={projectId}
      />
      {error ? (
        <p className="px-2 text-caption-sm-regular text-danger-primary">{error}</p>
      ) : (
        property.is_required &&
        values.length === 0 && (
          <p className="px-2 text-caption-sm-regular text-tertiary">{t("common.errors.required")}</p>
        )
      )}
    </SidebarPropertyListItem>
  );
});

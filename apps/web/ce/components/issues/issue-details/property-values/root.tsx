/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { CalendarDays, Info, Link2, Mail, Paperclip, Type } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import {
  BooleanPropertyIcon,
  DropdownPropertyIcon,
  HashPropertyIcon,
  MembersPropertyIcon,
  RelationPropertyIcon,
} from "@plane/propel/icons";
import { Tooltip } from "@plane/propel/tooltip";
import type { TIssueProperty, TIssuePropertyValue } from "@plane/types";
// components
import { SidebarPropertyListItem } from "@/components/common/layout/sidebar/property-list-item";
// hooks
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web hooks
import { useIssuePropertyValues } from "@/plane-web/hooks/store";
// local imports
import { WorkItemBooleanPropertyValue } from "./boolean-value";
import { WorkItemDatetimePropertyValue } from "./datetime-value";
import { WorkItemMemberPropertyValue } from "./member-value";
import { WorkItemOptionPropertyValue } from "./option-value";
import { WorkItemTextPropertyValue } from "./text-value";
import type { TWorkItemPropertyValueProps } from "./types";
import { WorkItemRelationPropertyValue } from "./work-item-value";

type TWorkItemPropertyValueRootProps = {
  property: TIssueProperty;
  workspaceSlug: string;
  projectId: string;
  workItemId: string;
  isEditable: boolean;
};

const propertyIcon = (property: TIssueProperty) => {
  switch (property.property_type) {
    case "URL":
      return Link2;
    case "EMAIL":
      return Mail;
    case "DECIMAL":
      return HashPropertyIcon;
    case "BOOLEAN":
      return BooleanPropertyIcon;
    case "DATETIME":
      return CalendarDays;
    case "OPTION":
      return DropdownPropertyIcon;
    case "FILE":
      return Paperclip;
    case "RELATION":
      return property.relation_type === "USER" ? MembersPropertyIcon : RelationPropertyIcon;
    default:
      return Type;
  }
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

  const editorProps: TWorkItemPropertyValueProps = {
    property,
    values,
    disabled: !isEditable,
    hasError: Boolean(error),
    onChange: handleChange,
  };

  const renderEditor = () => {
    switch (property.property_type) {
      case "BOOLEAN":
        return <WorkItemBooleanPropertyValue {...editorProps} />;
      case "DATETIME":
        return <WorkItemDatetimePropertyValue {...editorProps} />;
      case "OPTION":
        return <WorkItemOptionPropertyValue {...editorProps} />;
      case "RELATION":
        return property.relation_type === "USER" ? (
          <WorkItemMemberPropertyValue {...editorProps} workspaceSlug={workspaceSlug} projectId={projectId} />
        ) : (
          <WorkItemRelationPropertyValue {...editorProps} workspaceSlug={workspaceSlug} projectId={projectId} />
        );
      // files are stored as an asset id the sidebar has no uploader for yet
      case "FILE":
        return (
          <span className="flex h-7.5 items-center px-2 text-body-xs-regular text-placeholder">
            {t("work_item_properties.unsupported")}
          </span>
        );
      default:
        return <WorkItemTextPropertyValue {...editorProps} />;
    }
  };

  return (
    <SidebarPropertyListItem
      icon={propertyIcon(property)}
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
      {renderEditor()}
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

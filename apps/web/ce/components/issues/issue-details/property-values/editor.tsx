/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { CalendarDays, Link2, Mail, Paperclip, Type } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import {
  BooleanPropertyIcon,
  DropdownPropertyIcon,
  HashPropertyIcon,
  MembersPropertyIcon,
  RelationPropertyIcon,
} from "@plane/propel/icons";
import type { TIssueProperty } from "@plane/types";
// local imports
import { WorkItemBooleanPropertyValue } from "./boolean-value";
import { WorkItemDatetimePropertyValue } from "./datetime-value";
import { WorkItemMemberPropertyValue } from "./member-value";
import { WorkItemOptionPropertyValue } from "./option-value";
import { WorkItemTextPropertyValue } from "./text-value";
import type { TWorkItemRelationPropertyValueProps } from "./types";
import { WorkItemRelationPropertyValue } from "./work-item-value";

/** The icon standing for a property type, next to its label. */
export const workItemPropertyIcon = (property: TIssueProperty) => {
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

/**
 * The editor a property type is edited with. Fully controlled — the sidebar writes
 * through to the API on every change, the create/update modal holds the value in
 * form state until the work item is saved.
 */
export const WorkItemPropertyValueEditor = observer(function WorkItemPropertyValueEditor(
  props: TWorkItemRelationPropertyValueProps
) {
  const { property, workspaceSlug, projectId, ...editorProps } = props;
  // plane hooks
  const { t } = useTranslation();

  switch (property.property_type) {
    case "BOOLEAN":
      return <WorkItemBooleanPropertyValue property={property} {...editorProps} />;
    case "DATETIME":
      return <WorkItemDatetimePropertyValue property={property} {...editorProps} />;
    case "OPTION":
      return <WorkItemOptionPropertyValue property={property} {...editorProps} />;
    case "RELATION":
      return property.relation_type === "USER" ? (
        <WorkItemMemberPropertyValue
          property={property}
          {...editorProps}
          workspaceSlug={workspaceSlug}
          projectId={projectId}
        />
      ) : (
        <WorkItemRelationPropertyValue
          property={property}
          {...editorProps}
          workspaceSlug={workspaceSlug}
          projectId={projectId}
        />
      );
    // files are stored as an asset id there is no uploader for yet
    case "FILE":
      return (
        <span className="flex h-7.5 items-center px-2 text-body-xs-regular text-placeholder">
          {t("work_item_properties.unsupported")}
        </span>
      );
    default:
      return <WorkItemTextPropertyValue property={property} {...editorProps} />;
  }
});

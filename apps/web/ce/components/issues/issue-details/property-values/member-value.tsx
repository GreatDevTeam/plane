/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { cn } from "@plane/utils";
// components
import { MemberDropdown } from "@/components/dropdowns/member/dropdown";
// local imports
import type { TWorkItemRelationPropertyValueProps } from "./types";

/** The editor of a `RELATION` property that points at workspace members. */
export const WorkItemMemberPropertyValue = observer(function WorkItemMemberPropertyValue(
  props: TWorkItemRelationPropertyValueProps
) {
  const { property, values, disabled, hasError, onChange, projectId } = props;
  // plane hooks
  const { t } = useTranslation();
  // derived values
  const selectedIds = values.map((value) => String(value));
  const placeholder = t("work_item_properties.select_member");
  const sharedProps = {
    projectId,
    disabled,
    placeholder,
    className: "group w-full grow",
    buttonContainerClassName: "w-full text-left h-7.5",
    buttonClassName: cn("text-body-xs-regular", {
      "text-placeholder": selectedIds.length === 0,
      "text-danger-primary": hasError,
    }),
    hideIcon: selectedIds.length === 0,
    dropdownArrow: true,
    dropdownArrowClassName: "h-3.5 w-3.5 hidden group-hover:inline",
  };

  if (property.is_multi) {
    return (
      <MemberDropdown
        {...sharedProps}
        multiple
        value={selectedIds}
        onChange={(nextValues) => onChange(nextValues)}
        buttonVariant={selectedIds.length > 1 ? "transparent-without-text" : "transparent-with-text"}
      />
    );
  }

  return (
    <MemberDropdown
      {...sharedProps}
      multiple={false}
      value={selectedIds[0] ?? null}
      onChange={(nextValue) => onChange(nextValue ? [nextValue] : [])}
      buttonVariant="transparent-with-text"
    />
  );
});

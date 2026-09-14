/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import type { Control, FieldPath, FieldValues } from "react-hook-form";
import { Controller } from "react-hook-form";
// plane imports
import { ETabIndices } from "@plane/constants";
import type { EditorRefApi } from "@plane/editor";
// types
import type { TBulkIssueProperties, TIssue } from "@plane/types";
import { cn, getTabIndex } from "@plane/utils";
// hooks
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web imports
import { IssueTypeDropdown } from "@/plane-web/components/issues/issue-type-dropdown";
import { useIssueTypes } from "@/plane-web/hooks/store";

export type TIssueFields = TIssue & TBulkIssueProperties;

export type TIssueTypeDropdownVariant = "xs" | "sm";

export type TIssueTypeSelectProps<T extends Partial<TIssueFields>> = {
  control: Control<T>;
  projectId: string | null;
  editorRef?: React.MutableRefObject<EditorRefApi | null>;
  disabled?: boolean;
  variant?: TIssueTypeDropdownVariant;
  placeholder?: string;
  isRequired?: boolean;
  renderChevron?: boolean;
  dropDownContainerClassName?: string;
  showMandatoryFieldInfo?: boolean; // Show info about mandatory fields
  handleFormChange?: () => void;
};

type TIssueTypeSelectDropdownProps = {
  disabled: boolean;
  dropDownContainerClassName: string | undefined;
  onChange: (issueTypeId: string) => void;
  placeholder: string | undefined;
  projectId: string | null;
  renderChevron: boolean;
  value: string | null | undefined;
};

/**
 * Kept separate from the generic wrapper below: `observer` collapses generics, so the
 * observable reads live here where the props are concrete.
 */
const IssueTypeSelectDropdown = observer(function IssueTypeSelectDropdown(props: TIssueTypeSelectDropdownProps) {
  const { disabled, dropDownContainerClassName, onChange, placeholder, projectId, renderChevron, value } = props;
  // store hooks
  const { isMobile } = usePlatformOS();
  const { getProjectIssueTypes } = useIssueTypes();
  // derived values
  const issueTypes = getProjectIssueTypes(projectId);
  const { getIndex } = getTabIndex(ETabIndices.ISSUE_FORM, isMobile);

  // a project with a single work item type has nothing to pick from
  if (issueTypes.length < 2) return <></>;

  return (
    <div className={cn("h-7", dropDownContainerClassName)}>
      <IssueTypeDropdown
        value={value}
        onChange={onChange}
        projectId={projectId}
        buttonVariant="border-with-text"
        dropdownArrow={renderChevron}
        placeholder={placeholder}
        tabIndex={getIndex("type_id")}
        disabled={disabled}
      />
    </div>
  );
});

export function IssueTypeSelect<T extends Partial<TIssueFields>>(props: TIssueTypeSelectProps<T>) {
  const {
    control,
    projectId,
    disabled = false,
    placeholder,
    isRequired = false,
    renderChevron = false,
    dropDownContainerClassName,
    handleFormChange,
  } = props;

  return (
    <Controller
      control={control}
      name={"type_id" as FieldPath<T & FieldValues>}
      rules={{ required: isRequired }}
      render={({ field: { value, onChange } }) => (
        <IssueTypeSelectDropdown
          value={value as string | null | undefined}
          onChange={(issueTypeId) => {
            onChange(issueTypeId);
            handleFormChange?.();
          }}
          projectId={projectId}
          disabled={disabled}
          placeholder={placeholder}
          renderChevron={renderChevron}
          dropDownContainerClassName={dropDownContainerClassName}
        />
      )}
    />
  );
}

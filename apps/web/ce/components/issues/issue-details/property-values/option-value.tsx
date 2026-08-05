/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { ChevronDownIcon } from "@plane/propel/icons";
import type { ICustomSearchSelectOption } from "@plane/types";
import { CustomSearchSelect } from "@plane/ui";
import { cn } from "@plane/utils";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";
// local imports
import type { TWorkItemPropertyValueProps } from "./types";

/** The value of a single valued property that stands for "no option". */
const NO_OPTION = "";

export const WorkItemOptionPropertyValue = observer(function WorkItemOptionPropertyValue(
  props: TWorkItemPropertyValueProps
) {
  const { property, values, disabled, hasError, onChange } = props;
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getPropertyOptions } = useIssueProperties();
  // derived values
  const options = getPropertyOptions(property.id);
  const selectedIds = values.map((value) => String(value));
  const selectedNames = selectedIds
    .map((optionId) => options.find((option) => option.id === optionId)?.name)
    .filter(Boolean);

  const dropdownOptions: ICustomSearchSelectOption[] = options.map((option) => ({
    value: option.id,
    query: option.name,
    content: <span className="grow truncate">{option.name}</span>,
  }));

  const label = (
    <span
      className={cn("grow truncate text-left text-body-xs-regular", {
        "text-placeholder": selectedNames.length === 0,
        "text-danger-primary": hasError,
      })}
    >
      {selectedNames.length > 0 ? selectedNames.join(", ") : t("work_item_properties.select_option")}
    </span>
  );

  const customButton = (
    <span className="flex h-7.5 w-full items-center gap-1 rounded-sm px-2 hover:bg-layer-1">
      {label}
      {!disabled && <ChevronDownIcon className="hidden size-3.5 shrink-0 group-hover:inline" aria-hidden="true" />}
    </span>
  );

  if (property.is_multi) {
    return (
      <CustomSearchSelect
        className="group w-full grow"
        customButtonClassName="w-full"
        customButton={customButton}
        options={dropdownOptions}
        value={selectedIds}
        onChange={(nextValues: string[]) => onChange(nextValues)}
        disabled={disabled}
        multiple
        noResultsMessage={t("no_matching_results")}
        maxHeight="lg"
      />
    );
  }

  return (
    <CustomSearchSelect
      className="group w-full grow"
      customButtonClassName="w-full"
      customButton={customButton}
      // a value is cleared by picking "None" — a single valued combobox has no other way out
      options={
        property.is_required
          ? dropdownOptions
          : [
              {
                value: NO_OPTION,
                query: t("common.none"),
                content: <span className="grow truncate text-placeholder">{t("common.none")}</span>,
              },
              ...dropdownOptions,
            ]
      }
      value={selectedIds[0] ?? NO_OPTION}
      onChange={(nextValue: string) => onChange(nextValue === NO_OPTION ? [] : [nextValue])}
      disabled={disabled}
      noResultsMessage={t("no_matching_results")}
      maxHeight="lg"
    />
  );
});

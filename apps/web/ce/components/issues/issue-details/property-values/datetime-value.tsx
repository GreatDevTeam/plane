/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { cn, getDate, renderFormattedPayloadDate } from "@plane/utils";
// components
import { DateDropdown } from "@/components/dropdowns/date";
// local imports
import type { TWorkItemPropertyValueProps } from "./types";

/**
 * The API stores a datetime and hands it back as an ISO string, while the picker works in
 * whole days — so a picked day is sent as its UTC midnight and reads back as the same day
 * whatever the reader's offset is. A multi valued property keeps one empty picker at the
 * end to add the next date with.
 */
export const WorkItemDatetimePropertyValue = observer(function WorkItemDatetimePropertyValue(
  props: TWorkItemPropertyValueProps
) {
  const { property, values, disabled, hasError, onChange } = props;
  // plane hooks
  const { t } = useTranslation();
  // derived values
  const dates = values.map((value) => String(value));
  const rows = property.is_multi ? [...dates, ""] : [dates[0] ?? ""];

  const handleChange = (index: number, date: Date | null) => {
    const day = renderFormattedPayloadDate(date);
    const nextValues = [...dates];
    if (day) nextValues[index] = `${day}T00:00:00Z`;
    else nextValues.splice(index, 1);
    onChange(nextValues.filter((value, position) => value && nextValues.indexOf(value) === position));
  };

  return (
    <div className="w-full space-y-1">
      {rows.map((date, index) => (
        // the values are deduplicated, so at most one row — the trailing empty one — has no date
        <DateDropdown
          key={date || "empty"}
          value={getDate(date) ?? null}
          onChange={(value) => handleChange(index, value)}
          disabled={disabled}
          placeholder={t("work_item_properties.empty")}
          buttonVariant="transparent-with-text"
          className="group w-full grow"
          buttonContainerClassName="w-full text-left h-7.5"
          buttonClassName={cn("text-body-xs-regular", {
            "text-placeholder": !date,
            "text-danger-primary": hasError,
          })}
          hideIcon
          clearIconClassName="h-3 w-3 hidden group-hover:inline"
        />
      ))}
    </div>
  );
});

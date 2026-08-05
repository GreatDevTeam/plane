/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useEffect, useState } from "react";
import { isEqual } from "lodash-es";
import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { CloseIcon } from "@plane/propel/icons";
import { Input } from "@plane/propel/input";
import type { TIssuePropertyType, TIssuePropertyValue } from "@plane/types";
// local imports
import type { TWorkItemPropertyValueProps } from "./types";

const INPUT_TYPE_BY_PROPERTY_TYPE: Partial<Record<TIssuePropertyType, string>> = {
  EMAIL: "email",
  URL: "url",
  DECIMAL: "number",
};

/**
 * A multi valued property always keeps one empty row at the end to type the next value
 * into; a single valued one is exactly one row.
 */
const toDrafts = (values: TIssuePropertyValue[], isMulti: boolean) => {
  const drafts = values.map((value) => String(value));
  return isMulti ? [...drafts, ""] : [drafts[0] ?? ""];
};

/** The editor of every property whose value is typed into an input — text, url, email and decimal. */
export const WorkItemTextPropertyValue = observer(function WorkItemTextPropertyValue(
  props: TWorkItemPropertyValueProps
) {
  const { property, values, disabled, hasError, onChange } = props;
  // plane hooks
  const { t } = useTranslation();
  // states
  const [drafts, setDrafts] = useState<string[]>(() => toDrafts(values, property.is_multi));
  // the store hands back a fresh list on every write, so the drafts follow its contents
  // rather than its identity
  const valuesKey = JSON.stringify(values);

  useEffect(() => {
    setDrafts(toDrafts(values, property.is_multi));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [valuesKey, property.is_multi]);

  const handleCommit = (nextDrafts: string[]) => {
    const filled = nextDrafts.map((draft) => draft.trim()).filter((draft) => draft.length > 0);
    const parsed: TIssuePropertyValue[] =
      property.property_type === "DECIMAL" ? filled.map(Number).filter((value) => !Number.isNaN(value)) : filled;
    const nextValues = parsed.filter((value, index) => parsed.indexOf(value) === index);

    // nothing to save — drop whatever whitespace or duplicate the drafts picked up
    if (isEqual(nextValues, [...values])) {
      setDrafts(toDrafts(values, property.is_multi));
      return;
    }
    onChange(nextValues);
  };

  return (
    <div className="w-full space-y-1">
      {drafts.map((draft, index) => (
        // the list is rebuilt from the stored values after every commit, so the index is stable
        // eslint-disable-next-line react/no-array-index-key
        <div key={index} className="flex w-full items-center gap-1">
          <Input
            type={INPUT_TYPE_BY_PROPERTY_TYPE[property.property_type] ?? "text"}
            value={draft}
            mode="transparent"
            inputSize="xs"
            hasError={hasError}
            disabled={disabled}
            placeholder={t("work_item_properties.empty")}
            className="h-7.5 w-full grow text-body-xs-regular"
            min={property.settings?.min}
            max={property.settings?.max}
            maxLength={property.settings?.max_length}
            onChange={(event) =>
              setDrafts(drafts.map((value, position) => (position === index ? event.target.value : value)))
            }
            onBlur={() => handleCommit(drafts)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                event.currentTarget.blur();
              }
            }}
          />
          {property.is_multi && !disabled && draft.trim().length > 0 && (
            <button
              type="button"
              className="shrink-0 rounded-sm p-1 text-tertiary hover:text-danger-primary"
              aria-label={t("work_item_properties.remove_value")}
              onClick={() => handleCommit(drafts.filter((_, position) => position !== index))}
            >
              <CloseIcon className="size-3" />
            </button>
          )}
        </div>
      ))}
    </div>
  );
});

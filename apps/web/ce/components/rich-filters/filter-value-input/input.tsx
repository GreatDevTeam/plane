/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import React, { useEffect, useRef, useState } from "react";
import { observer } from "mobx-react";
// plane imports
import { cn } from "@plane/utils";
// local imports
import { COMMON_FILTER_ITEM_BORDER_CLASSNAME, EMPTY_FILTER_PLACEHOLDER_TEXT } from "@/components/rich-filters/shared";

export type TFilterTypedValueInputProps = {
  value: string;
  type: "text" | "number";
  isDisabled?: boolean;
  min?: number;
  max?: number;
  placeholder?: string;
  onChange: (value: string) => void;
};

/** How long the user stops typing for before the board is re-fetched. */
const COMMIT_DELAY_MS = 500;

/**
 * The editor behind the text and number filters — one input, committed once the user
 * stops typing rather than on every keystroke, since each commit re-runs the query.
 *
 * The draft follows the condition whenever the condition changes from the outside (a
 * saved view being loaded, the filter being cleared) but not while the user is typing.
 */
export const FilterTypedValueInput = observer(function FilterTypedValueInput(props: TFilterTypedValueInputProps) {
  const { value, type, isDisabled, min, max, placeholder, onChange } = props;
  // states
  const [draft, setDraft] = useState(value);
  // the timer the pending commit is on, cleared whenever another keystroke lands
  const commitTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const committed = useRef(value);

  useEffect(() => {
    if (value === committed.current) return;
    committed.current = value;
    setDraft(value);
  }, [value]);

  useEffect(() => () => clearTimeout(commitTimer.current), []);

  const handleChange = (next: string) => {
    setDraft(next);
    clearTimeout(commitTimer.current);
    commitTimer.current = setTimeout(() => {
      committed.current = next;
      onChange(next);
    }, COMMIT_DELAY_MS);
  };

  const handleCommitNow = () => {
    clearTimeout(commitTimer.current);
    if (draft === committed.current) return;
    committed.current = draft;
    onChange(draft);
  };

  return (
    <input
      type={type}
      value={draft}
      min={min}
      max={max}
      disabled={isDisabled}
      placeholder={placeholder ?? EMPTY_FILTER_PLACEHOLDER_TEXT}
      onChange={(event) => handleChange(event.target.value)}
      onBlur={handleCommitNow}
      onKeyDown={(event) => {
        if (event.key === "Enter") handleCommitNow();
      }}
      // the filter bar is a row of segments, so the input carries the segment border
      // rather than one of its own
      className={cn(
        "h-full w-24 bg-transparent px-2 text-11 outline-none placeholder:text-placeholder",
        { [COMMON_FILTER_ITEM_BORDER_CLASSNAME]: !isDisabled },
        { "cursor-not-allowed": isDisabled }
      )}
      // a filter that was just added has an empty input and nothing else to do in it,
      // the same way the date filter opens its picker straight away
      // eslint-disable-next-line jsx-a11y/no-autofocus
      autoFocus={!value}
    />
  );
});

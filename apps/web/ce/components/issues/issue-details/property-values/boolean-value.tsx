/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
// plane imports
import { ToggleSwitch } from "@plane/ui";
// local imports
import type { TWorkItemPropertyValueProps } from "./types";

/**
 * A boolean holds a single value — `is_multi` has no meaning for it, and both states are
 * stored, so a toggle that is switched off reads back as `false` rather than as unset.
 */
export const WorkItemBooleanPropertyValue = observer(function WorkItemBooleanPropertyValue(
  props: TWorkItemPropertyValueProps
) {
  const { property, values, disabled, onChange } = props;
  const value = values[0] === true || values[0] === "true";

  return (
    <div className="flex h-7.5 items-center">
      <ToggleSwitch
        value={value}
        onChange={(nextValue) => onChange([nextValue])}
        disabled={disabled}
        label={property.display_name}
        size="sm"
      />
    </div>
  );
});

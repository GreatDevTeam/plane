/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useCallback } from "react";
import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import { ToggleSwitch } from "@plane/ui";
// components
import { SettingsControlItem } from "@/components/settings/control-item";
// hooks
import { useUserProfile } from "@/hooks/store/user";

type Props = {
  option: {
    id: string;
    title: string;
    description: string;
  };
};

export const JiraStyleSwitcher = observer(function JiraStyleSwitcher(props: Props) {
  const { option } = props;
  // store hooks
  const { data: userProfile, updateUserProfile } = useUserProfile();
  // translation
  const { t } = useTranslation();

  const handleChange = useCallback(
    async (value: boolean) => {
      // the store swallows the rejection and rolls the optimistic update back,
      // so an undefined result is how a failed update surfaces here
      const updatedProfile = await updateUserProfile({ is_jira_style_enabled: value });
      if (!updatedProfile) {
        setToast({
          type: TOAST_TYPE.ERROR,
          title: "Error!",
          message: "Failed to update Jira style. Please try again.",
        });
      }
    },
    [updateUserProfile]
  );

  if (!userProfile) return null;

  return (
    <SettingsControlItem
      title={t(option.title)}
      description={t(option.description)}
      control={
        <ToggleSwitch
          value={!!userProfile.is_jira_style_enabled}
          onChange={(value) => {
            void handleChange(value);
          }}
          size="md"
        />
      }
    />
  );
});

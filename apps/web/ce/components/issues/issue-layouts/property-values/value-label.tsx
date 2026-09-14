/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useTranslation } from "@plane/i18n";
import type { TIssueProperty, TIssuePropertyValue } from "@plane/types";
import { renderFormattedDate } from "@plane/utils";
// hooks
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
import { useMember } from "@/hooks/store/use-member";
import { useProject } from "@/hooks/store/use-project";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";

/**
 * The values of one property as the text a card renders them with. Read only — a card
 * has no room for an editor, so the sidebar and the create/update modal stay the places
 * a custom property is edited.
 *
 * Must be called from an `observer`: the option, member and work item names it resolves
 * are read off the stores that hold them.
 */
export const useWorkItemPropertyValueLabel = (property: TIssueProperty, values: TIssuePropertyValue[]): string => {
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getPropertyOptionById } = useIssueProperties();
  const { getUserDetails } = useMember();
  const { getProjectById } = useProject();
  const {
    issue: { getIssueById },
  } = useIssueDetail();

  if (values.length === 0) return "";

  switch (property.property_type) {
    case "BOOLEAN":
      return values[0] === true || values[0] === "true" ? t("common.yes") : t("common.no");
    case "DATETIME":
      return values.map((value) => renderFormattedDate(String(value)) ?? String(value)).join(", ");
    case "OPTION":
      return values
        .map((value) => getPropertyOptionById(String(value))?.name)
        .filter(Boolean)
        .join(", ");
    case "RELATION":
      if (property.relation_type === "USER") {
        return values
          .map((value) => getUserDetails(String(value))?.display_name)
          .filter(Boolean)
          .join(", ");
      }
      // a work item is named by its identifier once it has been loaded; a card does not
      // load the ones it does not itself render, so what is left over is counted the way
      // the built-in sub work item and link chips count theirs
      {
        const identifiers = values
          .map((value) => {
            const workItem = getIssueById(String(value));
            const projectDetails = getProjectById(workItem?.project_id);
            return workItem && projectDetails ? `${projectDetails.identifier}-${workItem.sequence_id}` : undefined;
          })
          .filter(Boolean);
        return identifiers.length === values.length ? identifiers.join(", ") : String(values.length);
      }
    // a file is stored as an asset id there is nothing to render it with yet
    case "FILE":
      return "";
    default:
      return values.map((value) => String(value)).join(", ");
  }
};

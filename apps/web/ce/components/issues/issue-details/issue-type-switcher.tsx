/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { useParams } from "next/navigation";
// plane imports
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
// store hooks
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
// plane web components
import { IssueIdentifier } from "@/plane-web/components/issues/issue-details/issue-identifier";
import { IssueTypeDropdown } from "@/plane-web/components/issues/issue-type-dropdown";
// plane web hooks
import { useIssueTypes } from "@/plane-web/hooks/store";

export type TIssueTypeSwitcherProps = {
  issueId: string;
  disabled: boolean;
};

export const IssueTypeSwitcher = observer(function IssueTypeSwitcher(props: TIssueTypeSwitcherProps) {
  const { issueId, disabled } = props;
  // router params
  const { workspaceSlug } = useParams();
  // store hooks
  const {
    issue: { getIssueById },
    updateIssue,
  } = useIssueDetail();
  const { getProjectIssueTypes } = useIssueTypes();
  // derived values
  const issue = getIssueById(issueId);
  const issueTypes = getProjectIssueTypes(issue?.project_id);

  if (!issue || !issue.project_id) return <></>;

  // with a single type there is nothing to switch between — render the plain identifier
  if (disabled || issueTypes.length < 2) {
    return <IssueIdentifier issueId={issueId} projectId={issue.project_id} size="md" enableClickToCopyIdentifier />;
  }

  const handleIssueTypeChange = async (typeId: string) => {
    if (!workspaceSlug || !issue.project_id || typeId === issue.type_id) return;
    try {
      await updateIssue(workspaceSlug.toString(), issue.project_id, issueId, { type_id: typeId });
    } catch {
      setToast({
        type: TOAST_TYPE.ERROR,
        title: "Error!",
        message: "Work item type could not be changed. Please try again.",
      });
    }
  };

  return (
    <div className="flex items-center gap-2">
      <div className="h-7">
        <IssueTypeDropdown
          value={issue.type_id}
          onChange={handleIssueTypeChange}
          projectId={issue.project_id}
          buttonVariant="border-with-text"
          dropdownArrow
          showTooltip
        />
      </div>
      {/* the type is already shown by the dropdown, so only the key is rendered here */}
      <IssueIdentifier
        issueId={issueId}
        projectId={issue.project_id}
        size="md"
        displayProperties={{ key: true, issue_type: false }}
        enableClickToCopyIdentifier
      />
    </div>
  );
});

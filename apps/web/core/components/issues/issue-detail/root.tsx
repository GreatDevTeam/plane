/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useMemo, useRef } from "react";
import { observer } from "mobx-react";
// plane imports
import { EUserPermissions, EUserPermissionsLevel } from "@plane/constants";
import { useTranslation } from "@plane/i18n";
import { ChevronDownIcon, ChevronUpIcon } from "@plane/propel/icons";
import { IconButton } from "@plane/propel/icon-button";
import { TOAST_TYPE, setPromiseToast, setToast } from "@plane/propel/toast";
import type { TIssue } from "@plane/types";
import { EIssuesStoreType } from "@plane/types";
import { cn } from "@plane/utils";
// assets
import emptyIssue from "@/app/assets/empty-state/issue.svg?url";
// components
import { EmptyState } from "@/components/common/empty-state";
// hooks
import { useAppTheme } from "@/hooks/store/use-app-theme";
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
import { useIssues } from "@/hooks/store/use-issues";
import { useUserPermissions } from "@/hooks/store/user";
import { useAppRouter } from "@/hooks/use-app-router";
// local components
import { IssuePeekOverview } from "../peek-overview";
import { IssueMainContent } from "./main-content";
import { IssueDetailsSidebar } from "./sidebar";

export type TIssueOperations = {
  fetch: (workspaceSlug: string, projectId: string, issueId: string, loader?: boolean) => Promise<void>;
  update: (workspaceSlug: string, projectId: string, issueId: string, data: Partial<TIssue>) => Promise<void>;
  remove: (workspaceSlug: string, projectId: string, issueId: string) => Promise<void>;
  archive?: (workspaceSlug: string, projectId: string, issueId: string) => Promise<void>;
  restore?: (workspaceSlug: string, projectId: string, issueId: string) => Promise<void>;
  addCycleToIssue?: (workspaceSlug: string, projectId: string, cycleId: string, issueId: string) => Promise<void>;
  addIssueToCycle?: (workspaceSlug: string, projectId: string, cycleId: string, issueIds: string[]) => Promise<void>;
  removeIssueFromCycle?: (workspaceSlug: string, projectId: string, cycleId: string, issueId: string) => Promise<void>;
  removeIssueFromModule?: (
    workspaceSlug: string,
    projectId: string,
    moduleId: string,
    issueId: string
  ) => Promise<void>;
  changeModulesInIssue?: (
    workspaceSlug: string,
    projectId: string,
    issueId: string,
    addModuleIds: string[],
    removeModuleIds: string[]
  ) => Promise<void>;
};

export type TIssueDetailRoot = {
  workspaceSlug: string;
  projectId: string;
  issueId: string;
  is_archived?: boolean;
};

export const IssueDetailRoot = observer(function IssueDetailRoot(props: TIssueDetailRoot) {
  const { t } = useTranslation();
  const { workspaceSlug, projectId, issueId, is_archived = false } = props;
  // router
  const router = useAppRouter();
  // hooks
  const {
    issue: { getIssueById },
    fetchIssue,
    updateIssue,
    removeIssue,
    archiveIssue,
    addCycleToIssue,
    addIssueToCycle,
    removeIssueFromCycle,
    changeModulesInIssue,
    removeIssueFromModule,
  } = useIssueDetail();
  const {
    issues: { removeIssue: removeArchivedIssue },
  } = useIssues(EIssuesStoreType.ARCHIVED);
  const { allowPermissions } = useUserPermissions();
  const { issueDetailSidebarCollapsed } = useAppTheme();
  // refs
  const contentScrollRef = useRef<HTMLDivElement>(null);

  const scrollContentToTop = () => contentScrollRef.current?.scrollTo({ top: 0, behavior: "smooth" });
  const scrollContentToBottom = () =>
    contentScrollRef.current?.scrollTo({ top: contentScrollRef.current.scrollHeight, behavior: "smooth" });

  const issueOperations: TIssueOperations = useMemo(
    () => ({
      fetch: async (opWorkspaceSlug: string, opProjectId: string, opIssueId: string) => {
        try {
          await fetchIssue(opWorkspaceSlug, opProjectId, opIssueId);
        } catch (error) {
          console.error("Error fetching the parent issue:", error);
        }
      },
      update: async (opWorkspaceSlug: string, opProjectId: string, opIssueId: string, data: Partial<TIssue>) => {
        try {
          await updateIssue(opWorkspaceSlug, opProjectId, opIssueId, data);
        } catch (error) {
          console.log("Error in updating issue:", error);
          setToast({
            title: t("common.error.label"),
            type: TOAST_TYPE.ERROR,
            message: t("entity.update.failed", { entity: t("issue.label") }),
          });
        }
      },
      remove: async (opWorkspaceSlug: string, opProjectId: string, opIssueId: string) => {
        try {
          if (is_archived) await removeArchivedIssue(opWorkspaceSlug, opProjectId, opIssueId);
          else await removeIssue(opWorkspaceSlug, opProjectId, opIssueId);
          setToast({
            title: t("common.success"),
            type: TOAST_TYPE.SUCCESS,
            message: t("entity.delete.success", { entity: t("issue.label") }),
          });
        } catch (error) {
          console.log("Error in deleting issue:", error);
          setToast({
            title: t("common.error.label"),
            type: TOAST_TYPE.ERROR,
            message: t("entity.delete.failed", { entity: t("issue.label") }),
          });
        }
      },
      archive: async (opWorkspaceSlug: string, opProjectId: string, opIssueId: string) => {
        try {
          await archiveIssue(opWorkspaceSlug, opProjectId, opIssueId);
        } catch (error) {
          console.log("Error in archiving issue:", error);
        }
      },
      addCycleToIssue: async (opWorkspaceSlug: string, opProjectId: string, cycleId: string, opIssueId: string) => {
        try {
          await addCycleToIssue(opWorkspaceSlug, opProjectId, cycleId, opIssueId);
        } catch (_error) {
          setToast({
            type: TOAST_TYPE.ERROR,
            title: t("common.error.label"),
            message: t("issue.add.cycle.failed"),
          });
        }
      },
      addIssueToCycle: async (opWorkspaceSlug: string, opProjectId: string, cycleId: string, issueIds: string[]) => {
        try {
          await addIssueToCycle(opWorkspaceSlug, opProjectId, cycleId, issueIds);
        } catch (_error) {
          setToast({
            type: TOAST_TYPE.ERROR,
            title: t("common.error.label"),
            message: t("issue.add.cycle.failed"),
          });
        }
      },
      removeIssueFromCycle: async (
        opWorkspaceSlug: string,
        opProjectId: string,
        cycleId: string,
        opIssueId: string
      ) => {
        try {
          const removeFromCyclePromise = removeIssueFromCycle(opWorkspaceSlug, opProjectId, cycleId, opIssueId);
          setPromiseToast(removeFromCyclePromise, {
            loading: t("issue.remove.cycle.loading"),
            success: {
              title: t("common.success"),
              message: () => t("issue.remove.cycle.success"),
            },
            error: {
              title: t("common.error.label"),
              message: () => t("issue.remove.cycle.failed"),
            },
          });
          await removeFromCyclePromise;
        } catch (error) {
          console.log("Error in removing issue from cycle:", error);
        }
      },
      removeIssueFromModule: async (
        opWorkspaceSlug: string,
        opProjectId: string,
        moduleId: string,
        opIssueId: string
      ) => {
        try {
          const removeFromModulePromise = removeIssueFromModule(opWorkspaceSlug, opProjectId, moduleId, opIssueId);
          setPromiseToast(removeFromModulePromise, {
            loading: t("issue.remove.module.loading"),
            success: {
              title: t("common.success"),
              message: () => t("issue.remove.module.success"),
            },
            error: {
              title: t("common.error.label"),
              message: () => t("issue.remove.module.failed"),
            },
          });
          await removeFromModulePromise;
        } catch (error) {
          console.log("Error in removing issue from module:", error);
        }
      },
      changeModulesInIssue: async (
        opWorkspaceSlug: string,
        opProjectId: string,
        opIssueId: string,
        addModuleIds: string[],
        removeModuleIds: string[]
      ) => {
        const promise = await changeModulesInIssue(
          opWorkspaceSlug,
          opProjectId,
          opIssueId,
          addModuleIds,
          removeModuleIds
        );
        return promise;
      },
    }),
    [
      is_archived,
      fetchIssue,
      updateIssue,
      removeIssue,
      archiveIssue,
      removeArchivedIssue,
      addIssueToCycle,
      addCycleToIssue,
      removeIssueFromCycle,
      changeModulesInIssue,
      removeIssueFromModule,
      t,
    ]
  );

  // issue details
  const issue = getIssueById(issueId);
  // checking if issue is editable, based on user role
  const isEditable = allowPermissions(
    [EUserPermissions.ADMIN, EUserPermissions.MEMBER],
    EUserPermissionsLevel.PROJECT,
    workspaceSlug,
    projectId
  );

  return (
    <>
      {!issue ? (
        <EmptyState
          image={emptyIssue}
          title={t("issue.empty_state.issue_detail.title")}
          description={t("issue.empty_state.issue_detail.description")}
          primaryButton={{
            text: t("issue.empty_state.issue_detail.primary_button.text"),
            onClick: () => router.push(`/${workspaceSlug}/projects/${projectId}/issues`),
          }}
        />
      ) : (
        <div className="flex h-full w-full overflow-hidden">
          <div ref={contentScrollRef} className="h-full w-full space-y-6 overflow-y-auto px-9 py-5">
            <IssueMainContent
              workspaceSlug={workspaceSlug}
              projectId={projectId}
              issueId={issueId}
              issueOperations={issueOperations}
              isEditable={isEditable}
              isArchived={is_archived}
            />
          </div>
          <div
            className={cn("fixed right-4 bottom-4 z-[4] flex flex-col gap-2 md:hidden", {
              hidden: !issueDetailSidebarCollapsed,
            })}
          >
            <IconButton
              icon={ChevronUpIcon}
              variant="secondary"
              size="xl"
              className="rounded-full shadow-raised-100"
              aria-label="Scroll to top"
              onClick={scrollContentToTop}
            />
            <IconButton
              icon={ChevronDownIcon}
              variant="secondary"
              size="xl"
              className="rounded-full shadow-raised-100"
              aria-label="Scroll to bottom"
              onClick={scrollContentToBottom}
            />
          </div>
          <div
            className="fixed right-0 z-[5] h-full w-full min-w-[300px] border-l border-subtle bg-surface-1 sm:w-1/2 md:relative md:w-1/4 lg:min-w-80 xl:min-w-96"
            style={issueDetailSidebarCollapsed ? { right: `-${window?.innerWidth || 0}px` } : {}}
          >
            <IssueDetailsSidebar
              workspaceSlug={workspaceSlug}
              projectId={projectId}
              issueId={issueId}
              issueOperations={issueOperations}
              isEditable={!is_archived && isEditable}
            />
          </div>
        </div>
      )}

      {/* peek overview */}
      <IssuePeekOverview />
    </>
  );
});

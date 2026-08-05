/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { CloseIcon, EditIcon } from "@plane/propel/icons";
import { Tooltip } from "@plane/propel/tooltip";
import type { ISearchIssueResponse } from "@plane/types";
import { cn, generateWorkItemLink } from "@plane/utils";
// components
import { ExistingIssuesListModal } from "@/components/core/modals/existing-issues-list-modal";
// hooks
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
import { useProject } from "@/hooks/store/use-project";
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web hooks
import { useIssuePropertyValues } from "@/plane-web/hooks/store";
// local imports
import type { TWorkItemRelationPropertyValueProps } from "./types";

/**
 * The editor of a `RELATION` property that points at work items. The API only accepts work
 * items of the same project, so the picker stays project scoped and the referenced work
 * items can be loaded in one request.
 */
export const WorkItemRelationPropertyValue = observer(function WorkItemRelationPropertyValue(
  props: TWorkItemRelationPropertyValueProps
) {
  const { property, values, disabled, hasError, onChange, workspaceSlug, projectId } = props;
  // states
  const [isModalOpen, setIsModalOpen] = useState(false);
  // plane hooks
  const { t } = useTranslation();
  const { isMobile } = usePlatformOS();
  // store hooks
  const { getProjectById } = useProject();
  const {
    issue: { getIssueById },
  } = useIssueDetail();
  const { fetchRelatedWorkItems } = useIssuePropertyValues();
  // derived values
  const selectedIds = values.map((value) => String(value));
  const selectedIdsKey = selectedIds.join(",");

  // the values are bare ids — the chips need the work items they point at
  useEffect(() => {
    if (selectedIds.length === 0) return;
    fetchRelatedWorkItems(workspaceSlug, projectId, selectedIds).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedIdsKey, workspaceSlug, projectId]);

  const handleSubmit = async (data: ISearchIssueResponse[]) => {
    const pickedIds = data.map((workItem) => workItem.id);
    const nextValues = property.is_multi
      ? [...selectedIds, ...pickedIds.filter((workItemId) => !selectedIds.includes(workItemId))]
      : pickedIds.slice(0, 1);
    onChange(nextValues);
  };

  return (
    <>
      <ExistingIssuesListModal
        workspaceSlug={workspaceSlug}
        projectId={projectId}
        isOpen={isModalOpen}
        handleClose={() => setIsModalOpen(false)}
        searchParams={{}}
        handleOnSubmit={handleSubmit}
      />

      {/* not a single button — the chips carry their own link and remove control */}
      <div className="group flex w-full items-start gap-2 rounded-sm px-2 py-1">
        {selectedIds.length > 0 ? (
          <div className="flex grow flex-wrap items-center gap-1.5">
            {selectedIds.map((workItemId) => {
              const workItem = getIssueById(workItemId);
              const projectDetails = getProjectById(workItem?.project_id);

              return (
                <span
                  key={workItemId}
                  className="flex items-center gap-1 rounded-sm bg-layer-1 px-1.5 py-0.5 text-caption-sm-medium"
                >
                  {workItem && projectDetails ? (
                    <Tooltip tooltipHeading={t("common.title")} tooltipContent={workItem.name} isMobile={isMobile}>
                      <Link
                        href={generateWorkItemLink({
                          workspaceSlug,
                          projectId: projectDetails.id,
                          issueId: workItem.id,
                          projectIdentifier: projectDetails.identifier,
                          sequenceId: workItem.sequence_id,
                        })}
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={(event) => event.stopPropagation()}
                      >
                        {`${projectDetails.identifier}-${workItem.sequence_id}`}
                      </Link>
                    </Tooltip>
                  ) : (
                    <span className="text-tertiary">{t("common.loading")}</span>
                  )}
                  {!disabled && (
                    <Tooltip tooltipContent={t("common.remove")} position="bottom" isMobile={isMobile}>
                      <button
                        type="button"
                        aria-label={t("common.remove")}
                        onClick={() => onChange(selectedIds.filter((id) => id !== workItemId))}
                      >
                        <CloseIcon className="size-2.5 text-tertiary hover:text-danger-primary" />
                      </button>
                    </Tooltip>
                  )}
                </span>
              );
            })}
          </div>
        ) : (
          <button
            type="button"
            className={cn("grow text-left text-body-xs-regular text-placeholder", {
              "cursor-not-allowed": disabled,
              "text-danger-primary": hasError,
            })}
            onClick={() => setIsModalOpen(true)}
            disabled={disabled}
          >
            {t("work_item_properties.select_work_item")}
          </button>
        )}
        {!disabled && (
          <button
            type="button"
            aria-label={t("work_item_properties.select_work_item")}
            className="shrink-0 p-1 opacity-0 group-hover:opacity-100"
            onClick={() => setIsModalOpen(true)}
          >
            <EditIcon className="size-2.5 shrink-0" />
          </button>
        )}
      </div>
    </>
  );
});

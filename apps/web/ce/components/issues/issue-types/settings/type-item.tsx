/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useState } from "react";
import { observer } from "mobx-react";
import { ChevronDown, ChevronRight, Pencil, Trash2 } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type { TIssueType } from "@plane/types";
import { AlertModalCore, ToggleSwitch } from "@plane/ui";
// plane web hooks
import { useIssueProperties, useIssueTypes } from "@/plane-web/hooks/store";
// local imports
import { WorkItemTypePropertyList } from "./property-list";
import { WorkItemTypeForm } from "./type-form";

type TWorkItemTypeItemProps = {
  workspaceSlug: string;
  projectId: string;
  issueType: TIssueType;
  disabled: boolean;
};

/** One work item type — its name, whether it is on, and the fields it carries. */
export const WorkItemTypeItem = observer(function WorkItemTypeItem(props: TWorkItemTypeItemProps) {
  const { workspaceSlug, projectId, issueType, disabled } = props;
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { updateIssueType, deleteIssueType } = useIssueTypes();
  const { ensureIssueTypeProperties } = useIssueProperties();
  // states
  const [isExpanded, setIsExpanded] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const reportError = (error: unknown) => {
    const data = error as { error?: string; detail?: string } | undefined;
    setToast({
      type: TOAST_TYPE.ERROR,
      title: t("common.error.label"),
      message: data?.error ?? data?.detail ?? t("common.something_went_wrong"),
    });
  };

  const handleToggleExpanded = () => {
    // the properties of a type are only read once, when it is first opened
    if (!isExpanded) ensureIssueTypeProperties(workspaceSlug, issueType.id);
    setIsExpanded((expanded) => !expanded);
  };

  const handleToggleActive = async (isActive: boolean) => {
    try {
      await updateIssueType(workspaceSlug, projectId, issueType.id, { is_active: isActive });
    } catch (error) {
      reportError(error);
    }
  };

  const handleDelete = async () => {
    setIsDeleting(true);
    try {
      await deleteIssueType(workspaceSlug, projectId, issueType.id);
      setIsDeleteOpen(false);
    } catch (error) {
      reportError(error);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <>
      <AlertModalCore
        isOpen={isDeleteOpen}
        handleClose={() => setIsDeleteOpen(false)}
        handleSubmit={handleDelete}
        isSubmitting={isDeleting}
        title={t("project_settings.work_item_types.delete_type")}
        content={t("project_settings.work_item_types.delete_type_content")}
      />
      <div className="rounded-sm border border-subtle">
        {isEditing ? (
          <WorkItemTypeForm
            issueType={issueType}
            onSubmit={(data) => updateIssueType(workspaceSlug, projectId, issueType.id, data)}
            onClose={() => setIsEditing(false)}
          />
        ) : (
          <div className="flex items-center gap-2 px-3 py-2.5">
            <button
              type="button"
              onClick={handleToggleExpanded}
              aria-expanded={isExpanded}
              aria-label={t("project_settings.work_item_types.fields")}
              className="grid size-5 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
            >
              {isExpanded ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
            </button>
            <span className="truncate text-14 font-medium text-primary">{issueType.name}</span>
            {issueType.is_default && (
              <span className="rounded-sm bg-layer-2 px-1.5 py-0.5 text-caption-sm-medium text-secondary">
                {t("project_settings.work_item_types.default_type")}
              </span>
            )}
            {issueType.is_epic && (
              <span className="rounded-sm bg-layer-2 px-1.5 py-0.5 text-caption-sm-medium text-secondary">
                {t("project_settings.work_item_types.epic_type")}
              </span>
            )}
            <span className="truncate text-caption-sm-regular text-tertiary">{issueType.description}</span>
            <span className="flex-1" />
            {!disabled && (
              <>
                <ToggleSwitch
                  value={issueType.is_active}
                  onChange={handleToggleActive}
                  label={t("project_settings.work_item_types.active")}
                />
                <button
                  type="button"
                  onClick={() => setIsEditing(true)}
                  aria-label={t("edit")}
                  className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
                >
                  <Pencil className="size-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => setIsDeleteOpen(true)}
                  aria-label={t("project_settings.work_item_types.delete_type")}
                  className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2 hover:text-danger-primary"
                >
                  <Trash2 className="size-3.5" />
                </button>
              </>
            )}
          </div>
        )}
        {isExpanded && !isEditing && (
          <div className="border-t border-subtle px-3 py-3 pl-10">
            <WorkItemTypePropertyList workspaceSlug={workspaceSlug} issueTypeId={issueType.id} disabled={disabled} />
          </div>
        )}
      </div>
    </>
  );
});

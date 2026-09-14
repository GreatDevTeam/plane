/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useState } from "react";
import { observer } from "mobx-react";
import { Check, Pencil, Trash2, X } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Button } from "@plane/propel/button";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type { TIssuePropertyOption } from "@plane/types";
import { AlertModalCore, Input } from "@plane/ui";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";

type TWorkItemPropertyOptionListProps = {
  workspaceSlug: string;
  propertyId: string;
  disabled: boolean;
};

/** Reads the message the API rejected the write with, falling back to a generic one. */
const errorMessage = (error: unknown, fallback: string): string => {
  const data = error as { name?: string[]; error?: string; detail?: string } | undefined;
  return data?.name?.[0] ?? data?.error ?? data?.detail ?? fallback;
};

/** The choices of one `OPTION` field — added, renamed, defaulted and deleted in place. */
export const WorkItemPropertyOptionList = observer(function WorkItemPropertyOptionList(
  props: TWorkItemPropertyOptionListProps
) {
  const { workspaceSlug, propertyId, disabled } = props;
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getAllPropertyOptions, createPropertyOption, updatePropertyOption, deletePropertyOption } =
    useIssueProperties();
  // states
  const [newOptionName, setNewOptionName] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [editingOptionId, setEditingOptionId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [optionToDelete, setOptionToDelete] = useState<TIssuePropertyOption | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  // derived values
  const options = getAllPropertyOptions(propertyId);

  const handleCreate = async () => {
    const name = newOptionName.trim();
    if (!name || isSubmitting) return;

    setIsSubmitting(true);
    try {
      await createPropertyOption(workspaceSlug, propertyId, { name });
      setNewOptionName("");
    } catch (error) {
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: errorMessage(error, t("common.something_went_wrong")),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRename = async (optionId: string) => {
    const name = editingName.trim();
    if (!name || isSubmitting) return;

    setIsSubmitting(true);
    try {
      await updatePropertyOption(workspaceSlug, propertyId, optionId, { name });
      setEditingOptionId(null);
    } catch (error) {
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: errorMessage(error, t("common.something_went_wrong")),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSetDefault = async (option: TIssuePropertyOption) => {
    try {
      await updatePropertyOption(workspaceSlug, propertyId, option.id, { is_default: !option.is_default });
    } catch (error) {
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: errorMessage(error, t("common.something_went_wrong")),
      });
    }
  };

  const handleDelete = async () => {
    if (!optionToDelete) return;

    setIsDeleting(true);
    try {
      await deletePropertyOption(workspaceSlug, propertyId, optionToDelete.id);
      setOptionToDelete(null);
    } catch (error) {
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: errorMessage(error, t("common.something_went_wrong")),
      });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <>
      <AlertModalCore
        isOpen={!!optionToDelete}
        handleClose={() => setOptionToDelete(null)}
        handleSubmit={handleDelete}
        isSubmitting={isDeleting}
        title={t("project_settings.work_item_types.delete_option")}
        content={t("project_settings.work_item_types.delete_option_content")}
      />
      <div className="flex flex-col gap-1.5">
        <span className="text-caption-sm-medium text-secondary">{t("project_settings.work_item_types.options")}</span>
        {options.length === 0 && (
          <p className="text-caption-sm-regular text-tertiary">{t("project_settings.work_item_types.no_options")}</p>
        )}
        {options.map((option) =>
          editingOptionId === option.id ? (
            <div key={option.id} className="flex items-center gap-2">
              <Input
                value={editingName}
                onChange={(e) => setEditingName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleRename(option.id);
                  if (e.key === "Escape") setEditingOptionId(null);
                }}
                // oxlint-disable-next-line no-autofocus -- the form opens on a click, so the field it opens for is where focus belongs
                autoFocus
                className="h-8 w-full"
              />
              <Button variant="primary" size="sm" onClick={() => handleRename(option.id)} loading={isSubmitting}>
                {t("update")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setEditingOptionId(null)}>
                {t("cancel")}
              </Button>
            </div>
          ) : (
            <div key={option.id} className="flex items-center gap-2 rounded-sm border border-subtle px-3 py-1.5">
              <span className="flex-1 truncate text-14 text-primary">{option.name}</span>
              {option.is_default && (
                <span className="rounded-sm bg-layer-2 px-1.5 py-0.5 text-caption-sm-medium text-secondary">
                  {t("project_settings.work_item_types.default_option")}
                </span>
              )}
              {!disabled && (
                <>
                  <button
                    type="button"
                    onClick={() => handleSetDefault(option)}
                    title={t("project_settings.work_item_types.set_as_default")}
                    aria-label={t("project_settings.work_item_types.set_as_default")}
                    className={`grid size-6 place-items-center rounded-sm hover:bg-layer-2 ${
                      option.is_default ? "text-accent-primary" : "text-tertiary"
                    }`}
                  >
                    <Check className="size-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setEditingOptionId(option.id);
                      setEditingName(option.name);
                    }}
                    aria-label={t("edit")}
                    className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
                  >
                    <Pencil className="size-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => setOptionToDelete(option)}
                    aria-label={t("project_settings.work_item_types.delete_option")}
                    className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2 hover:text-danger-primary"
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </>
              )}
            </div>
          )
        )}
        {!disabled && (
          <div className="flex items-center gap-2">
            <Input
              value={newOptionName}
              onChange={(e) => setNewOptionName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  handleCreate();
                }
              }}
              placeholder={t("project_settings.work_item_types.option_name")}
              className="h-8 w-full"
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCreate}
              loading={isSubmitting}
              disabled={!newOptionName.trim()}
            >
              {t("project_settings.work_item_types.add_option")}
            </Button>
            {newOptionName && (
              <button
                type="button"
                onClick={() => setNewOptionName("")}
                aria-label={t("cancel")}
                className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>
        )}
      </div>
    </>
  );
});

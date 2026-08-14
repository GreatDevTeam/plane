/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useState } from "react";
import { observer } from "mobx-react";
import { ChevronDown, ChevronRight, Pencil, Plus, Trash2 } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Button } from "@plane/propel/button";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type { TIssueProperty } from "@plane/types";
import { AlertModalCore } from "@plane/ui";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";
// local imports
import { workItemPropertyIcon } from "../../issue-details/property-values";
import { WorkItemPropertyOptionList } from "./option-list";
import { WorkItemPropertyForm } from "./property-form";

type TWorkItemTypePropertyListProps = {
  workspaceSlug: string;
  issueTypeId: string;
  disabled: boolean;
};

/** The custom fields of one work item type — defined, edited and deleted in place. */
export const WorkItemTypePropertyList = observer(function WorkItemTypePropertyList(
  props: TWorkItemTypePropertyListProps
) {
  const { workspaceSlug, issueTypeId, disabled } = props;
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getIssueTypeProperties, createIssueProperty, updateIssueProperty, deleteIssueProperty } =
    useIssueProperties();
  // states
  const [isCreating, setIsCreating] = useState(false);
  const [editingPropertyId, setEditingPropertyId] = useState<string | null>(null);
  const [expandedPropertyId, setExpandedPropertyId] = useState<string | null>(null);
  const [propertyToDelete, setPropertyToDelete] = useState<TIssueProperty | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  // derived values
  const properties = getIssueTypeProperties(issueTypeId);

  const handleDelete = async () => {
    if (!propertyToDelete) return;

    setIsDeleting(true);
    try {
      await deleteIssueProperty(workspaceSlug, issueTypeId, propertyToDelete.id);
      setPropertyToDelete(null);
    } catch (error) {
      const data = error as { error?: string } | undefined;
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: data?.error ?? t("common.something_went_wrong"),
      });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <>
      <AlertModalCore
        isOpen={!!propertyToDelete}
        handleClose={() => setPropertyToDelete(null)}
        handleSubmit={handleDelete}
        isSubmitting={isDeleting}
        title={t("project_settings.work_item_types.delete_field")}
        content={t("project_settings.work_item_types.delete_field_content")}
      />
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <span className="text-caption-sm-medium text-secondary">{t("project_settings.work_item_types.fields")}</span>
          {!disabled && !isCreating && (
            <Button
              variant="secondary"
              size="sm"
              prependIcon={<Plus className="size-3.5" />}
              onClick={() => {
                setEditingPropertyId(null);
                setIsCreating(true);
              }}
            >
              {t("project_settings.work_item_types.add_field")}
            </Button>
          )}
        </div>

        {isCreating && (
          <WorkItemPropertyForm
            onSubmit={(data) => createIssueProperty(workspaceSlug, issueTypeId, data)}
            onClose={() => setIsCreating(false)}
          />
        )}

        {properties.length === 0 && !isCreating && (
          <p className="text-caption-sm-regular text-tertiary">{t("project_settings.work_item_types.no_fields")}</p>
        )}

        {properties.map((property) => {
          const Icon = workItemPropertyIcon(property);
          const isExpanded = expandedPropertyId === property.id;

          if (editingPropertyId === property.id) {
            return (
              <WorkItemPropertyForm
                key={property.id}
                property={property}
                onSubmit={(data) => updateIssueProperty(workspaceSlug, issueTypeId, property.id, data)}
                onClose={() => setEditingPropertyId(null)}
              />
            );
          }

          return (
            <div key={property.id} className="rounded-sm border border-subtle">
              <div className="flex items-center gap-2 px-3 py-2">
                {property.property_type === "OPTION" ? (
                  <button
                    type="button"
                    onClick={() => setExpandedPropertyId(isExpanded ? null : property.id)}
                    aria-label={t("project_settings.work_item_types.options")}
                    aria-expanded={isExpanded}
                    className="grid size-5 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
                  >
                    {isExpanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                  </button>
                ) : (
                  <span className="size-5" />
                )}
                <Icon className="size-4 shrink-0 text-tertiary" />
                <span className="truncate text-14 text-primary">{property.display_name}</span>
                {property.is_required && <span className="text-danger-primary">*</span>}
                <span className="truncate text-caption-sm-regular text-tertiary">
                  {t(`project_settings.work_item_types.field_types.${property.property_type}`)}
                  {property.is_multi ? ` · ${t("project_settings.work_item_types.allow_multiple")}` : ""}
                </span>
                {!property.is_active && (
                  <span className="rounded-sm bg-layer-2 px-1.5 py-0.5 text-caption-sm-medium text-secondary">
                    {t("project_settings.work_item_types.inactive")}
                  </span>
                )}
                <span className="flex-1" />
                <code className="truncate text-caption-sm-regular text-tertiary">{property.name}</code>
                {!disabled && (
                  <>
                    <button
                      type="button"
                      onClick={() => {
                        setIsCreating(false);
                        setEditingPropertyId(property.id);
                      }}
                      aria-label={t("edit")}
                      className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2"
                    >
                      <Pencil className="size-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setPropertyToDelete(property)}
                      aria-label={t("project_settings.work_item_types.delete_field")}
                      className="grid size-6 place-items-center rounded-sm text-tertiary hover:bg-layer-2 hover:text-danger-primary"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </>
                )}
              </div>
              {property.property_type === "OPTION" && isExpanded && (
                <div className="border-t border-subtle px-3 py-2 pl-10">
                  <WorkItemPropertyOptionList
                    workspaceSlug={workspaceSlug}
                    propertyId={property.id}
                    disabled={disabled}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </>
  );
});

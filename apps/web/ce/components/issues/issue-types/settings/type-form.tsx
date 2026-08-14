/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import { Controller, useForm } from "react-hook-form";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Button } from "@plane/propel/button";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type { TIssueType, TIssueTypePayload } from "@plane/types";
import { Input } from "@plane/ui";

type TWorkItemTypeFormProps = {
  /** Absent when the form defines a new type. */
  issueType?: TIssueType;
  onSubmit: (data: TIssueTypePayload) => Promise<unknown>;
  onClose: () => void;
};

type TFormValues = {
  name: string;
  description: string;
};

/** Defines or renames one work item type. */
export const WorkItemTypeForm = observer(function WorkItemTypeForm(props: TWorkItemTypeFormProps) {
  const { issueType, onSubmit, onClose } = props;
  // plane hooks
  const { t } = useTranslation();
  // form info
  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<TFormValues>({
    defaultValues: { name: issueType?.name ?? "", description: issueType?.description ?? "" },
  });

  const handleFormSubmit = async (formData: TFormValues) => {
    try {
      await onSubmit({ name: formData.name.trim(), description: formData.description.trim() });
      onClose();
    } catch (error) {
      const data = error as Record<string, string[] | string> | undefined;
      const firstError = Object.values(data ?? {}).flat()[0];
      setToast({
        type: TOAST_TYPE.ERROR,
        title: t("common.error.label"),
        message: typeof firstError === "string" ? firstError : t("common.something_went_wrong"),
      });
    }
  };

  return (
    <form
      onSubmit={handleSubmit(handleFormSubmit)}
      className="flex flex-col gap-3 rounded-sm border border-subtle bg-layer-1 p-3"
    >
      <div className="flex flex-col gap-3 md:flex-row">
        <div className="flex flex-1 flex-col gap-1">
          <label className="text-caption-sm-medium text-secondary" htmlFor="work-item-type-name">
            {t("project_settings.work_item_types.type_name")}
          </label>
          <Controller
            control={control}
            name="name"
            rules={{ required: t("project_settings.work_item_types.type_name_is_required"), maxLength: 255 }}
            render={({ field: { value, onChange } }) => (
              <Input
                id="work-item-type-name"
                value={value}
                onChange={onChange}
                hasError={Boolean(errors.name)}
                // oxlint-disable-next-line no-autofocus -- the form opens on a click, so the field it opens for is where focus belongs
                autoFocus
                className="w-full"
              />
            )}
          />
          {errors.name?.message && <p className="text-caption-sm-regular text-danger-primary">{errors.name.message}</p>}
        </div>
        <div className="flex flex-1 flex-col gap-1">
          <label className="text-caption-sm-medium text-secondary" htmlFor="work-item-type-description">
            {t("project_settings.work_item_types.type_description")}
          </label>
          <Controller
            control={control}
            name="description"
            render={({ field: { value, onChange } }) => (
              <Input id="work-item-type-description" value={value} onChange={onChange} className="w-full" />
            )}
          />
        </div>
      </div>
      <div className="flex items-center justify-end gap-2">
        <Button variant="secondary" size="sm" type="button" onClick={onClose}>
          {t("cancel")}
        </Button>
        <Button variant="primary" size="sm" type="submit" loading={isSubmitting}>
          {issueType ? t("update") : t("add")}
        </Button>
      </div>
    </form>
  );
});

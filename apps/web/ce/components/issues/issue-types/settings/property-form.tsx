/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useState } from "react";
import { observer } from "mobx-react";
import { Controller, useForm } from "react-hook-form";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Button } from "@plane/propel/button";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type {
  TIssueProperty,
  TIssuePropertyPayload,
  TIssuePropertyRelationType,
  TIssuePropertySettings,
  TIssuePropertyType,
} from "@plane/types";
import { CustomSelect, Input, ToggleSwitch } from "@plane/ui";
// local imports
import {
  ISSUE_PROPERTY_RELATION_TYPES,
  ISSUE_PROPERTY_TYPES,
  propertyTakesLengthBound,
  propertyTakesMultipleValues,
  propertyTakesRangeBound,
  toPropertyKey,
} from "./helper";

type TWorkItemPropertyFormProps = {
  /** Absent when the form defines a new field. */
  property?: TIssueProperty;
  onSubmit: (data: TIssuePropertyPayload) => Promise<unknown>;
  onClose: () => void;
};

type TFormValues = {
  display_name: string;
  name: string;
  description: string;
  property_type: TIssuePropertyType;
  relation_type: TIssuePropertyRelationType | null;
  is_required: boolean;
  is_multi: boolean;
  is_active: boolean;
  min: string;
  max: string;
  max_length: string;
};

const asNumber = (value: string): number | undefined => {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
};

const defaultsFor = (property: TIssueProperty | undefined): TFormValues => ({
  display_name: property?.display_name ?? "",
  name: property?.name ?? "",
  description: property?.description ?? "",
  property_type: property?.property_type ?? "TEXT",
  relation_type: property?.relation_type ?? null,
  is_required: property?.is_required ?? false,
  is_multi: property?.is_multi ?? false,
  is_active: property?.is_active ?? true,
  min: property?.settings?.min !== undefined ? String(property.settings.min) : "",
  max: property?.settings?.max !== undefined ? String(property.settings.max) : "",
  max_length: property?.settings?.max_length !== undefined ? String(property.settings.max_length) : "",
});

/**
 * Defines or edits one custom field. `property_type` is fixed once the field exists —
 * the stored values sit in the column that matches the original type — so the type
 * dropdown is only offered while creating.
 */
export const WorkItemPropertyForm = observer(function WorkItemPropertyForm(props: TWorkItemPropertyFormProps) {
  const { property, onSubmit, onClose } = props;
  // plane hooks
  const { t } = useTranslation();
  // states — the api key follows the display name until it is edited by hand
  const [isKeyEdited, setIsKeyEdited] = useState(Boolean(property));
  // form info
  const {
    control,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<TFormValues>({ defaultValues: defaultsFor(property) });
  // derived values
  const isUpdating = Boolean(property);
  const propertyType = watch("property_type");

  const handleFormSubmit = async (formData: TFormValues) => {
    const settings: TIssuePropertySettings = {};
    if (propertyTakesRangeBound(formData.property_type)) {
      const min = asNumber(formData.min);
      const max = asNumber(formData.max);
      if (min !== undefined) settings.min = min;
      if (max !== undefined) settings.max = max;
    }
    if (propertyTakesLengthBound(formData.property_type)) {
      const maxLength = asNumber(formData.max_length);
      if (maxLength !== undefined) settings.max_length = maxLength;
    }

    const payload: TIssuePropertyPayload = {
      display_name: formData.display_name.trim(),
      name: (formData.name.trim() || toPropertyKey(formData.display_name)).slice(0, 255),
      description: formData.description.trim(),
      is_required: formData.is_required,
      is_active: formData.is_active,
      is_multi: propertyTakesMultipleValues(formData.property_type) ? formData.is_multi : false,
      settings,
    };
    // the type is immutable, so an update must not send it back
    if (!isUpdating) {
      payload.property_type = formData.property_type;
      payload.relation_type = formData.property_type === "RELATION" ? (formData.relation_type ?? "USER") : null;
    }

    try {
      await onSubmit(payload);
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
          <label className="text-caption-sm-medium text-secondary" htmlFor="property-display-name">
            {t("project_settings.work_item_types.field_name")}
          </label>
          <Controller
            control={control}
            name="display_name"
            rules={{ required: t("project_settings.work_item_types.field_name_is_required"), maxLength: 255 }}
            render={({ field: { value, onChange } }) => (
              <Input
                id="property-display-name"
                value={value}
                onChange={(e) => {
                  onChange(e.target.value);
                  if (!isKeyEdited) setValue("name", toPropertyKey(e.target.value));
                }}
                hasError={Boolean(errors.display_name)}
                // oxlint-disable-next-line no-autofocus -- the form opens on a click, so the field it opens for is where focus belongs
                autoFocus
                className="w-full"
              />
            )}
          />
          {errors.display_name?.message && (
            <p className="text-caption-sm-regular text-danger-primary">{errors.display_name.message}</p>
          )}
        </div>
        <div className="flex flex-1 flex-col gap-1">
          <label className="text-caption-sm-medium text-secondary" htmlFor="property-name">
            {t("project_settings.work_item_types.field_key")}
          </label>
          <Controller
            control={control}
            name="name"
            render={({ field: { value, onChange } }) => (
              <Input
                id="property-name"
                value={value}
                onChange={(e) => {
                  setIsKeyEdited(true);
                  onChange(e.target.value);
                }}
                className="w-full"
              />
            )}
          />
          <p className="text-caption-sm-regular text-tertiary">
            {t("project_settings.work_item_types.field_key_hint")}
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <label className="text-caption-sm-medium text-secondary" htmlFor="property-description">
          {t("project_settings.work_item_types.field_description")}
        </label>
        <Controller
          control={control}
          name="description"
          render={({ field: { value, onChange } }) => (
            <Input id="property-description" value={value} onChange={onChange} className="w-full" />
          )}
        />
      </div>

      <div className="flex flex-col gap-3 md:flex-row">
        <div className="flex flex-1 flex-col gap-1">
          <span className="text-caption-sm-medium text-secondary">
            {t("project_settings.work_item_types.field_type")}
          </span>
          {isUpdating ? (
            <p className="text-14 text-primary">
              {t(`project_settings.work_item_types.field_types.${propertyType}`)}
              <span className="ml-2 text-caption-sm-regular text-tertiary">
                {t("project_settings.work_item_types.type_is_immutable")}
              </span>
            </p>
          ) : (
            <Controller
              control={control}
              name="property_type"
              render={({ field: { value, onChange } }) => (
                <CustomSelect
                  value={value}
                  onChange={(nextType: TIssuePropertyType) => {
                    onChange(nextType);
                    if (nextType !== "RELATION") setValue("relation_type", null);
                    else if (!watch("relation_type")) setValue("relation_type", "USER");
                  }}
                  label={t(`project_settings.work_item_types.field_types.${value}`)}
                  buttonClassName="w-full justify-between"
                  input
                >
                  {ISSUE_PROPERTY_TYPES.map((type) => (
                    <CustomSelect.Option key={type} value={type}>
                      {t(`project_settings.work_item_types.field_types.${type}`)}
                    </CustomSelect.Option>
                  ))}
                </CustomSelect>
              )}
            />
          )}
        </div>
        {propertyType === "RELATION" && !isUpdating && (
          <div className="flex flex-1 flex-col gap-1">
            <span className="text-caption-sm-medium text-secondary">
              {t("project_settings.work_item_types.field_relates_to")}
            </span>
            <Controller
              control={control}
              name="relation_type"
              render={({ field: { value, onChange } }) => (
                <CustomSelect
                  value={value ?? "USER"}
                  onChange={onChange}
                  label={t(`project_settings.work_item_types.relation_types.${value ?? "USER"}`)}
                  buttonClassName="w-full justify-between"
                  input
                >
                  {ISSUE_PROPERTY_RELATION_TYPES.map((relationType) => (
                    <CustomSelect.Option key={relationType} value={relationType}>
                      {t(`project_settings.work_item_types.relation_types.${relationType}`)}
                    </CustomSelect.Option>
                  ))}
                </CustomSelect>
              )}
            />
          </div>
        )}
        {propertyTakesRangeBound(propertyType) && (
          <>
            <div className="flex flex-col gap-1">
              <label className="text-caption-sm-medium text-secondary" htmlFor="property-min">
                {t("project_settings.work_item_types.min")}
              </label>
              <Controller
                control={control}
                name="min"
                render={({ field: { value, onChange } }) => (
                  <Input id="property-min" type="number" value={value} onChange={onChange} className="w-28" />
                )}
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-caption-sm-medium text-secondary" htmlFor="property-max">
                {t("project_settings.work_item_types.max")}
              </label>
              <Controller
                control={control}
                name="max"
                render={({ field: { value, onChange } }) => (
                  <Input id="property-max" type="number" value={value} onChange={onChange} className="w-28" />
                )}
              />
            </div>
          </>
        )}
        {propertyTakesLengthBound(propertyType) && (
          <div className="flex flex-col gap-1">
            <label className="text-caption-sm-medium text-secondary" htmlFor="property-max-length">
              {t("project_settings.work_item_types.max_length")}
            </label>
            <Controller
              control={control}
              name="max_length"
              render={({ field: { value, onChange } }) => (
                <Input id="property-max-length" type="number" value={value} onChange={onChange} className="w-28" />
              )}
            />
          </div>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-6">
        <Controller
          control={control}
          name="is_required"
          render={({ field: { value, onChange } }) => (
            <div className="flex items-center gap-2">
              <ToggleSwitch value={value} onChange={onChange} />
              <span className="text-14 text-secondary">{t("project_settings.work_item_types.required")}</span>
            </div>
          )}
        />
        {propertyTakesMultipleValues(propertyType) && (
          <Controller
            control={control}
            name="is_multi"
            render={({ field: { value, onChange } }) => (
              <div className="flex items-center gap-2">
                <ToggleSwitch value={value} onChange={onChange} />
                <span className="text-14 text-secondary">{t("project_settings.work_item_types.allow_multiple")}</span>
              </div>
            )}
          />
        )}
        <Controller
          control={control}
          name="is_active"
          render={({ field: { value, onChange } }) => (
            <div className="flex items-center gap-2">
              <ToggleSwitch value={value} onChange={onChange} />
              <span className="text-14 text-secondary">{t("project_settings.work_item_types.active")}</span>
            </div>
          )}
        />
      </div>

      <div className="flex items-center justify-end gap-2">
        <Button variant="secondary" size="sm" type="button" onClick={onClose}>
          {t("cancel")}
        </Button>
        <Button variant="primary" size="sm" type="submit" loading={isSubmitting}>
          {isUpdating ? t("update") : t("add")}
        </Button>
      </div>
    </form>
  );
});

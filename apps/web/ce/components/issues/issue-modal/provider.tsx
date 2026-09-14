/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import React, { useCallback, useMemo, useState } from "react";
import { isEmpty } from "lodash-es";
import { observer } from "mobx-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { TOAST_TYPE, setToast } from "@plane/propel/toast";
import type { ISearchIssueResponse, TIssue, TIssuePropertyValue } from "@plane/types";
// components
import { IssueModalContext } from "@/components/issues/issue-modal/context";
import type {
  TActiveAdditionalPropertiesProps,
  TCreateUpdatePropertyValuesProps,
  THandleProjectEntitiesFetchProps,
  TPropertyValuesValidationProps,
} from "@/components/issues/issue-modal/context";
// hooks
import { useUser } from "@/hooks/store/user/user-user";
// plane web imports
import { useIssueProperties, useIssuePropertyValues, useIssueTypes } from "@/plane-web/hooks/store";
import type { TIssuePropertyValueErrors, TIssuePropertyValues } from "@/plane-web/types/issue-types";

export type TIssueModalProviderProps = {
  templateId?: string;
  dataForPreload?: Partial<TIssue>;
  allowedProjectIds?: string[];
  children: React.ReactNode;
};

/** A value the user has actually filled in — an empty string is a cleared field. */
const isFilledIn = (value: TIssuePropertyValue) => value !== "" && value !== null && value !== undefined;

// stable identities for the parts of the contract CE does not implement, so that the
// context value only changes when something in it actually did
const noop = () => {};
const noopAsync = () => Promise.resolve();

export const IssueModalProvider = observer(function IssueModalProvider(props: TIssueModalProviderProps) {
  const { children, allowedProjectIds } = props;
  // states
  const [selectedParentIssue, setSelectedParentIssue] = useState<ISearchIssueResponse | null>(null);
  const [issuePropertyValues, setIssuePropertyValues] = useState<TIssuePropertyValues>({});
  const [issuePropertyValueErrors, setIssuePropertyValueErrors] = useState<TIssuePropertyValueErrors>({});
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { projectsWithCreatePermissions } = useUser();
  const { fetchProjectIssueTypes, getProjectDefaultIssueType } = useIssueTypes();
  const { fetchIssueTypeProperties, getActiveIssueTypeProperties } = useIssueProperties();
  const { replacePropertyValues } = useIssuePropertyValues();
  // derived values
  const allowedProjectIdsValue = useMemo(
    () => allowedProjectIds ?? Object.keys(projectsWithCreatePermissions ?? {}),
    [allowedProjectIds, projectsWithCreatePermissions]
  );

  /** What a work item of this type starts out with, used to seed and to reset the form. */
  const defaultValuesOf = useCallback(
    (issueTypeId: string | null | undefined): TIssuePropertyValues =>
      Object.fromEntries(
        getActiveIssueTypeProperties(issueTypeId).map((property) => [property.id, property.default_value ?? []])
      ),
    [getActiveIssueTypeProperties]
  );

  /** A work item dropped into a project without a type gets that project's default one. */
  const getIssueTypeIdOnProjectChange = useCallback(
    (projectId: string) => getProjectDefaultIssueType(projectId)?.id ?? null,
    [getProjectDefaultIssueType]
  );

  /**
   * How many custom properties the form is showing — the modal sizes and scrolls its
   * middle section by it.
   */
  const getActiveAdditionalPropertiesLength = useCallback(
    (propertiesProps: TActiveAdditionalPropertiesProps) =>
      getActiveIssueTypeProperties(propertiesProps.watch("type_id")).length,
    [getActiveIssueTypeProperties]
  );

  /**
   * Blocks the submit when a required property is empty. The API enforces the same
   * rule, but the work item is created before its property values are written, so
   * catching it here is what keeps a half filled work item from being created at all.
   */
  const handlePropertyValuesValidation = useCallback(
    (validationProps: TPropertyValuesValidationProps) => {
      const properties = getActiveIssueTypeProperties(validationProps.watch("type_id"));

      const errors: TIssuePropertyValueErrors = {};
      properties.forEach((property) => {
        if (!property.is_required) return;
        const values = (issuePropertyValues[property.id] ?? []).filter(isFilledIn);
        if (values.length === 0) errors[property.id] = t("common.errors.required");
      });

      setIssuePropertyValueErrors(errors);
      return isEmpty(errors);
    },
    [getActiveIssueTypeProperties, issuePropertyValues, t]
  );

  /**
   * Writes the property values once the work item exists — it is created first, so
   * this always runs against a known id. A workspace draft keeps its values in its own
   * table until it is converted into a work item.
   */
  const handleCreateUpdatePropertyValues = useCallback(
    async (valuesProps: TCreateUpdatePropertyValuesProps) => {
      const { issueId, projectId, workspaceSlug, issueTypeId, isDraft } = valuesProps;
      if (!issueId || !projectId || !workspaceSlug || isEmpty(issuePropertyValues)) return;

      try {
        await replacePropertyValues(workspaceSlug, projectId, issueId, issuePropertyValues, isDraft);
        // "create more" keeps the modal open — the next work item starts from the defaults again
        setIssuePropertyValues(defaultValuesOf(issueTypeId));
        setIssuePropertyValueErrors({});
      } catch (error) {
        // a rejection comes back keyed by property id; anything else is not ours to render
        const rejections = Object.entries((error ?? {}) as Record<string, unknown>).filter(
          ([propertyId, message]) => typeof message === "string" && propertyId in issuePropertyValues
        ) as [string, string][];

        setIssuePropertyValueErrors(Object.fromEntries(rejections));
        setToast({
          type: TOAST_TYPE.ERROR,
          title: t("error"),
          message: rejections[0]?.[1] ?? t("work_item_properties.values_could_not_be_saved"),
        });
      }
    },
    [defaultValuesOf, issuePropertyValues, replacePropertyValues, t]
  );

  /** Loads what the modal needs to render a project's work item types and their fields. */
  const handleProjectEntitiesFetch = useCallback(
    async (entitiesProps: THandleProjectEntitiesFetchProps) => {
      const { workItemProjectId, workItemTypeId, workspaceSlug } = entitiesProps;
      if (!workspaceSlug || !workItemProjectId) return;

      await fetchProjectIssueTypes(workspaceSlug, workItemProjectId);
      if (workItemTypeId) await fetchIssueTypeProperties(workspaceSlug, workItemTypeId);
    },
    [fetchIssueTypeProperties, fetchProjectIssueTypes]
  );

  const contextValue = useMemo(
    () => ({
      allowedProjectIds: allowedProjectIdsValue,
      workItemTemplateId: null,
      setWorkItemTemplateId: noop,
      isApplyingTemplate: false,
      setIsApplyingTemplate: noop,
      selectedParentIssue,
      setSelectedParentIssue,
      issuePropertyValues,
      setIssuePropertyValues,
      issuePropertyValueErrors,
      setIssuePropertyValueErrors,
      getIssueTypeIdOnProjectChange,
      getActiveAdditionalPropertiesLength,
      handlePropertyValuesValidation,
      handleCreateUpdatePropertyValues,
      handleProjectEntitiesFetch,
      handleTemplateChange: noopAsync,
      handleConvert: noopAsync,
      handleCreateSubWorkItem: noopAsync,
    }),
    [
      allowedProjectIdsValue,
      selectedParentIssue,
      issuePropertyValues,
      issuePropertyValueErrors,
      getIssueTypeIdOnProjectChange,
      getActiveAdditionalPropertiesLength,
      handlePropertyValuesValidation,
      handleCreateUpdatePropertyValues,
      handleProjectEntitiesFetch,
    ]
  );

  return <IssueModalContext.Provider value={contextValue}>{children}</IssueModalContext.Provider>;
});

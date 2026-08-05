/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { set, unset } from "lodash-es";
import { action, makeObservable, observable, runInAction } from "mobx";
import { computedFn } from "mobx-utils";
// plane imports
import type { TIssuePropertyValue, TIssuePropertyValues } from "@plane/types";
// services
import { IssuePropertyService } from "@/services/issue";
// store
import type { CoreRootStore } from "@/store/root.store";

/** Returned for a property with no value — a stable identity, so that reading it in a
 * component does not look like a change on every render. */
const EMPTY_VALUES: TIssuePropertyValue[] = [];

export interface IIssuePropertyValuesStore {
  // observables
  valuesMap: Record<string, TIssuePropertyValues>;
  errorMap: Record<string, Record<string, string>>;
  fetchedMap: Record<string, boolean>;
  // computed actions
  getWorkItemPropertyValues: (workItemId: string | null | undefined) => TIssuePropertyValues | undefined;
  getPropertyValue: (workItemId: string | null | undefined, propertyId: string) => TIssuePropertyValue[];
  getPropertyError: (workItemId: string | null | undefined, propertyId: string) => string | undefined;
  // actions
  clearPropertyError: (workItemId: string, propertyId: string) => void;
  fetchWorkItemPropertyValues: (
    workspaceSlug: string,
    projectId: string,
    workItemId: string
  ) => Promise<TIssuePropertyValues>;
  fetchDraftPropertyValues: (workspaceSlug: string, draftId: string) => Promise<TIssuePropertyValues>;
  /** Writes every submitted property at once, as the create/update modal does on save. */
  replacePropertyValues: (
    workspaceSlug: string,
    projectId: string,
    workItemId: string,
    values: TIssuePropertyValues,
    isDraft?: boolean
  ) => Promise<TIssuePropertyValues>;
  updatePropertyValue: (
    workspaceSlug: string,
    projectId: string,
    workItemId: string,
    propertyId: string,
    values: TIssuePropertyValue[]
  ) => Promise<void>;
  /** Loads the work items a `RELATION` value points at, so its chips can be rendered. */
  fetchRelatedWorkItems: (workspaceSlug: string, projectId: string, workItemIds: string[]) => Promise<void>;
}

export class IssuePropertyValuesStore implements IIssuePropertyValuesStore {
  // observables
  valuesMap: Record<string, TIssuePropertyValues> = {};
  errorMap: Record<string, Record<string, string>> = {};
  fetchedMap: Record<string, boolean> = {};
  // root store
  rootStore: CoreRootStore;
  // services
  issuePropertyService: IssuePropertyService;

  constructor(_rootStore: CoreRootStore) {
    makeObservable(this, {
      valuesMap: observable,
      errorMap: observable,
      fetchedMap: observable,
      // actions
      clearPropertyError: action,
      fetchWorkItemPropertyValues: action,
      fetchDraftPropertyValues: action,
      replacePropertyValues: action,
      updatePropertyValue: action,
    });

    this.rootStore = _rootStore;
    this.issuePropertyService = new IssuePropertyService();
  }

  getWorkItemPropertyValues = computedFn((workItemId: string | null | undefined) => {
    if (!workItemId) return undefined;
    return this.valuesMap[workItemId];
  });

  getPropertyValue = computedFn((workItemId: string | null | undefined, propertyId: string) => {
    if (!workItemId) return EMPTY_VALUES;
    return this.valuesMap[workItemId]?.[propertyId] ?? EMPTY_VALUES;
  });

  getPropertyError = computedFn((workItemId: string | null | undefined, propertyId: string) => {
    if (!workItemId) return undefined;
    return this.errorMap[workItemId]?.[propertyId];
  });

  clearPropertyError = (workItemId: string, propertyId: string) => {
    if (!this.errorMap[workItemId]?.[propertyId]) return;
    runInAction(() => unset(this.errorMap, [workItemId, propertyId]));
  };

  fetchWorkItemPropertyValues = async (workspaceSlug: string, projectId: string, workItemId: string) => {
    const values = await this.issuePropertyService.getIssuePropertyValues(workspaceSlug, projectId, workItemId);
    runInAction(() => {
      set(this.valuesMap, [workItemId], values);
      set(this.fetchedMap, [workItemId], true);
    });
    return values;
  };

  fetchDraftPropertyValues = async (workspaceSlug: string, draftId: string) => {
    const values = await this.issuePropertyService.getDraftPropertyValues(workspaceSlug, draftId);
    runInAction(() => {
      set(this.valuesMap, [draftId], values);
      set(this.fetchedMap, [draftId], true);
    });
    return values;
  };

  /**
   * Writes every submitted property in one request, as the create/update modal does
   * once the work item exists. Unlike the sidebar there is nothing to roll back — the
   * values were only ever in the modal's form state — so a rejection is thrown on for
   * the modal to render against its own fields.
   */
  replacePropertyValues = async (
    workspaceSlug: string,
    projectId: string,
    workItemId: string,
    values: TIssuePropertyValues,
    isDraft: boolean = false
  ) => {
    const updatedValues = isDraft
      ? await this.issuePropertyService.updateDraftPropertyValues(workspaceSlug, workItemId, values)
      : await this.issuePropertyService.updateIssuePropertyValues(workspaceSlug, projectId, workItemId, values);

    runInAction(() => {
      set(this.valuesMap, [workItemId], updatedValues);
      set(this.fetchedMap, [workItemId], true);
      unset(this.errorMap, [workItemId]);
    });
    return updatedValues;
  };

  /**
   * Writes one property optimistically. The API replaces only the property it is
   * given and hands the whole map back, so the response is authoritative; a rejection
   * arrives as `{ "<property_id>": "<message>" }` and is kept on `errorMap` for the
   * editor to render inline, with the previous value restored.
   */
  updatePropertyValue = async (
    workspaceSlug: string,
    projectId: string,
    workItemId: string,
    propertyId: string,
    values: TIssuePropertyValue[]
  ) => {
    const previousValues = this.valuesMap[workItemId]?.[propertyId];

    runInAction(() => {
      set(this.valuesMap, [workItemId, propertyId], values);
      unset(this.errorMap, [workItemId, propertyId]);
    });

    try {
      const updatedValues = await this.issuePropertyService.updateIssuePropertyValues(
        workspaceSlug,
        projectId,
        workItemId,
        { [propertyId]: values }
      );
      runInAction(() => set(this.valuesMap, [workItemId], updatedValues));
    } catch (error) {
      runInAction(() => {
        if (previousValues === undefined) unset(this.valuesMap, [workItemId, propertyId]);
        else set(this.valuesMap, [workItemId, propertyId], previousValues);
        set(this.errorMap, [workItemId, propertyId], this.errorMessage(error, propertyId));
      });
      throw error;
    }
  };

  fetchRelatedWorkItems = async (workspaceSlug: string, projectId: string, workItemIds: string[]) => {
    const missingIds = workItemIds.filter((workItemId) => !this.rootStore.issue.issues.getIssueById(workItemId));
    if (missingIds.length === 0) return;
    await this.rootStore.issue.issues.getIssues(workspaceSlug, projectId, missingIds);
  };

  private errorMessage = (error: unknown, propertyId: string) => {
    const message = (error as Record<string, unknown> | undefined)?.[propertyId];
    return typeof message === "string" ? message : "The value could not be saved.";
  };
}

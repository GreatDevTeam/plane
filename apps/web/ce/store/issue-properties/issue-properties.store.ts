/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { set, sortBy } from "lodash-es";
import { action, makeObservable, observable, runInAction } from "mobx";
import { computedFn } from "mobx-utils";
// plane imports
import type { TIssueProperty, TIssuePropertyOption } from "@plane/types";
// services
import { IssuePropertyService } from "@/services/issue";
// store
import type { CoreRootStore } from "@/store/root.store";

/** Returned for a type whose properties have not been fetched — a stable identity, so
 * that reading it in a component does not look like a change on every render. */
const EMPTY_IDS: string[] = [];

export interface IIssuePropertiesStore {
  // observables
  propertyMap: Record<string, TIssueProperty>;
  optionMap: Record<string, TIssuePropertyOption>;
  propertyIdsByIssueTypeId: Record<string, string[]>;
  optionIdsByPropertyId: Record<string, string[]>;
  fetchedIssueTypeMap: Record<string, boolean>;
  // computed actions
  getPropertyById: (propertyId: string | null | undefined) => TIssueProperty | undefined;
  getIssueTypeProperties: (issueTypeId: string | null | undefined) => TIssueProperty[];
  getActiveIssueTypeProperties: (issueTypeId: string | null | undefined) => TIssueProperty[];
  getPropertyOptions: (propertyId: string | null | undefined) => TIssuePropertyOption[];
  getPropertyOptionById: (optionId: string | null | undefined) => TIssuePropertyOption | undefined;
  // fetch actions
  fetchIssueTypeProperties: (workspaceSlug: string, issueTypeId: string) => Promise<TIssueProperty[]>;
}

export class IssuePropertiesStore implements IIssuePropertiesStore {
  // observables
  propertyMap: Record<string, TIssueProperty> = {};
  optionMap: Record<string, TIssuePropertyOption> = {};
  propertyIdsByIssueTypeId: Record<string, string[]> = {};
  optionIdsByPropertyId: Record<string, string[]> = {};
  fetchedIssueTypeMap: Record<string, boolean> = {};
  // root store
  rootStore: CoreRootStore;
  // services
  issuePropertyService: IssuePropertyService;

  constructor(_rootStore: CoreRootStore) {
    makeObservable(this, {
      propertyMap: observable,
      optionMap: observable,
      propertyIdsByIssueTypeId: observable,
      optionIdsByPropertyId: observable,
      fetchedIssueTypeMap: observable,
      // actions
      fetchIssueTypeProperties: action,
    });

    this.rootStore = _rootStore;
    this.issuePropertyService = new IssuePropertyService();
  }

  getPropertyById = computedFn((propertyId: string | null | undefined) => {
    if (!propertyId) return undefined;
    return this.propertyMap[propertyId];
  });

  getIssueTypeProperties = computedFn((issueTypeId: string | null | undefined) => {
    if (!issueTypeId) return [];
    const propertyIds = this.propertyIdsByIssueTypeId[issueTypeId] ?? EMPTY_IDS;
    return sortBy(propertyIds.map((propertyId) => this.propertyMap[propertyId]).filter(Boolean), [
      "sort_order",
      "created_at",
    ]);
  });

  /** What the sidebar renders — a deactivated property keeps its values but is hidden. */
  getActiveIssueTypeProperties = computedFn((issueTypeId: string | null | undefined) =>
    this.getIssueTypeProperties(issueTypeId).filter((property) => property.is_active)
  );

  getPropertyOptions = computedFn((propertyId: string | null | undefined) => {
    if (!propertyId) return [];
    const optionIds = this.optionIdsByPropertyId[propertyId] ?? EMPTY_IDS;
    return sortBy(
      optionIds.map((optionId) => this.optionMap[optionId]).filter((option) => option?.is_active),
      ["sort_order", "created_at"]
    );
  });

  getPropertyOptionById = computedFn((optionId: string | null | undefined) => {
    if (!optionId) return undefined;
    return this.optionMap[optionId];
  });

  /**
   * Fetches the properties of a work item type together with the options of every
   * `OPTION` property among them — the sidebar cannot render an option value without
   * the option it points at.
   */
  fetchIssueTypeProperties = async (workspaceSlug: string, issueTypeId: string) => {
    const properties = await this.issuePropertyService.getIssueTypeProperties(workspaceSlug, issueTypeId);

    runInAction(() => {
      properties.forEach((property) => set(this.propertyMap, [property.id], property));
      set(
        this.propertyIdsByIssueTypeId,
        [issueTypeId],
        properties.map((property) => property.id)
      );
      set(this.fetchedIssueTypeMap, [issueTypeId], true);
    });

    const optionProperties = properties.filter((property) => property.property_type === "OPTION");
    await Promise.all(
      optionProperties.map(async (property) => {
        const options = await this.issuePropertyService.getIssuePropertyOptions(workspaceSlug, property.id);
        runInAction(() => {
          options.forEach((option) => set(this.optionMap, [option.id], option));
          set(
            this.optionIdsByPropertyId,
            [property.id],
            options.map((option) => option.id)
          );
        });
      })
    );

    return properties;
  };
}

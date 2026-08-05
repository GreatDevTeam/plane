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
import type { RootStore } from "@/plane-web/store/root.store";

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
  /** Fetches a work item type's properties once, however many callers ask for them. */
  ensureIssueTypeProperties: (workspaceSlug: string | null | undefined, issueTypeId: string | null | undefined) => void;
  /** The properties of every work item type the project has enabled. */
  fetchProjectProperties: (workspaceSlug: string, projectId: string) => Promise<TIssueProperty[]>;
  getProjectProperties: (projectId: string | null | undefined) => TIssueProperty[];
}

export class IssuePropertiesStore implements IIssuePropertiesStore {
  // observables
  propertyMap: Record<string, TIssueProperty> = {};
  optionMap: Record<string, TIssuePropertyOption> = {};
  propertyIdsByIssueTypeId: Record<string, string[]> = {};
  optionIdsByPropertyId: Record<string, string[]> = {};
  fetchedIssueTypeMap: Record<string, boolean> = {};
  // root store — the work item types the properties hang off live on the plane web store
  rootStore: RootStore;
  // services
  issuePropertyService: IssuePropertyService;
  // the types whose properties are already being read, so a re-render does not read them twice
  private inFlightIssueTypeIds: Set<string> = new Set();

  constructor(_rootStore: RootStore) {
    makeObservable(this, {
      propertyMap: observable,
      optionMap: observable,
      propertyIdsByIssueTypeId: observable,
      optionIdsByPropertyId: observable,
      fetchedIssueTypeMap: observable,
      // actions
      fetchIssueTypeProperties: action,
      fetchProjectProperties: action,
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
   * The active properties of every work item type the project has enabled, in the
   * order their columns and chips are rendered in.
   */
  getProjectProperties = computedFn((projectId: string | null | undefined) => {
    if (!projectId) return [];
    return this.rootStore.issueTypes
      .getProjectIssueTypes(projectId)
      .flatMap((issueType) => this.getActiveIssueTypeProperties(issueType.id));
  });

  /**
   * Called from render paths — a card only knows the type of the work item it renders,
   * and every card of that type asks for the same properties.
   */
  ensureIssueTypeProperties = (workspaceSlug: string | null | undefined, issueTypeId: string | null | undefined) => {
    if (!workspaceSlug || !issueTypeId) return;
    if (this.fetchedIssueTypeMap[issueTypeId] || this.inFlightIssueTypeIds.has(issueTypeId)) return;

    this.inFlightIssueTypeIds.add(issueTypeId);
    this.fetchIssueTypeProperties(workspaceSlug, issueTypeId)
      // a failed read is not remembered, so the next render tries again
      .catch(() => {})
      .finally(() => this.inFlightIssueTypeIds.delete(issueTypeId));
  };

  /**
   * What the spreadsheet and the display property toggles need: a saved `property_<uuid>`
   * toggle names a property without saying which type it belongs to, so the whole project
   * is loaded rather than a single type.
   */
  fetchProjectProperties = async (workspaceSlug: string, projectId: string) => {
    const issueTypes = await this.rootStore.issueTypes.fetchProjectIssueTypes(workspaceSlug, projectId);
    const properties = await Promise.all(
      issueTypes.map((issueType) =>
        this.fetchedIssueTypeMap[issueType.id]
          ? this.getIssueTypeProperties(issueType.id)
          : this.fetchIssueTypeProperties(workspaceSlug, issueType.id)
      )
    );
    return properties.flat();
  };

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

/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { set, sortBy } from "lodash-es";
import { action, makeObservable, observable, runInAction } from "mobx";
import { computedFn } from "mobx-utils";
// plane imports
import type { TIssueType, TIssueTypePayload } from "@plane/types";
// services
import { IssueTypeService } from "@/services/issue";
// store
import type { CoreRootStore } from "@/store/root.store";

export interface IIssueTypesStore {
  // observables
  issueTypeMap: Record<string, TIssueType>;
  fetchedMap: Record<string, boolean>;
  // computed actions
  getIssueTypeById: (issueTypeId: string | null | undefined) => TIssueType | undefined;
  getProjectIssueTypes: (projectId: string | null | undefined) => TIssueType[];
  getProjectIssueTypeIds: (projectId: string | null | undefined) => string[];
  getProjectDefaultIssueType: (projectId: string | null | undefined) => TIssueType | undefined;
  // fetch actions
  fetchWorkspaceIssueTypes: (workspaceSlug: string) => Promise<TIssueType[]>;
  fetchProjectIssueTypes: (workspaceSlug: string, projectId: string) => Promise<TIssueType[]>;
  // crud actions
  createIssueType: (workspaceSlug: string, projectId: string, data: TIssueTypePayload) => Promise<TIssueType>;
  updateIssueType: (
    workspaceSlug: string,
    projectId: string,
    issueTypeId: string,
    data: TIssueTypePayload
  ) => Promise<TIssueType>;
  deleteIssueType: (workspaceSlug: string, projectId: string, issueTypeId: string) => Promise<void>;
}

export class IssueTypesStore implements IIssueTypesStore {
  // observables
  issueTypeMap: Record<string, TIssueType> = {};
  fetchedMap: Record<string, boolean> = {};
  // root store
  rootStore: CoreRootStore;
  // services
  issueTypeService: IssueTypeService;

  constructor(_rootStore: CoreRootStore) {
    makeObservable(this, {
      issueTypeMap: observable,
      fetchedMap: observable,
      // actions
      fetchWorkspaceIssueTypes: action,
      fetchProjectIssueTypes: action,
      createIssueType: action,
      updateIssueType: action,
      deleteIssueType: action,
    });

    this.rootStore = _rootStore;
    this.issueTypeService = new IssueTypeService();
  }

  getIssueTypeById = computedFn((issueTypeId: string | null | undefined) => {
    if (!issueTypeId) return undefined;
    return this.issueTypeMap[issueTypeId];
  });

  getProjectIssueTypes = computedFn((projectId: string | null | undefined) => {
    if (!projectId) return [];
    return sortBy(
      Object.values(this.issueTypeMap).filter(
        (issueType) => issueType.is_active && issueType.project_ids?.includes(projectId)
      ),
      ["level", "name"]
    );
  });

  getProjectIssueTypeIds = computedFn((projectId: string | null | undefined) =>
    this.getProjectIssueTypes(projectId).map((issueType) => issueType.id)
  );

  getProjectDefaultIssueType = computedFn((projectId: string | null | undefined) => {
    const issueTypes = this.getProjectIssueTypes(projectId);
    return issueTypes.find((issueType) => issueType.is_default) ?? issueTypes[0];
  });

  fetchWorkspaceIssueTypes = async (workspaceSlug: string) => {
    const issueTypes = await this.issueTypeService.getWorkspaceIssueTypes(workspaceSlug);
    runInAction(() => {
      issueTypes.forEach((issueType) => set(this.issueTypeMap, [issueType.id], issueType));
      set(this.fetchedMap, [workspaceSlug], true);
    });
    return issueTypes;
  };

  fetchProjectIssueTypes = async (workspaceSlug: string, projectId: string) => {
    const issueTypes = await this.issueTypeService.getProjectIssueTypes(workspaceSlug, projectId);
    runInAction(() => {
      issueTypes.forEach((issueType) => set(this.issueTypeMap, [issueType.id], issueType));
      set(this.fetchedMap, [projectId], true);
    });
    return issueTypes;
  };

  createIssueType = async (workspaceSlug: string, projectId: string, data: TIssueTypePayload) => {
    const issueType = await this.issueTypeService.createIssueType(workspaceSlug, projectId, data);
    runInAction(() => set(this.issueTypeMap, [issueType.id], issueType));
    return issueType;
  };

  updateIssueType = async (workspaceSlug: string, projectId: string, issueTypeId: string, data: TIssueTypePayload) => {
    const originalIssueType = this.issueTypeMap[issueTypeId];
    try {
      runInAction(() => set(this.issueTypeMap, [issueTypeId], { ...originalIssueType, ...data }));
      const issueType = await this.issueTypeService.updateIssueType(workspaceSlug, projectId, issueTypeId, data);
      runInAction(() => set(this.issueTypeMap, [issueTypeId], issueType));
      return issueType;
    } catch (error) {
      runInAction(() => set(this.issueTypeMap, [issueTypeId], originalIssueType));
      throw error;
    }
  };

  deleteIssueType = async (workspaceSlug: string, projectId: string, issueTypeId: string) => {
    await this.issueTypeService.deleteIssueType(workspaceSlug, projectId, issueTypeId);
    runInAction(() => {
      const issueType = this.issueTypeMap[issueTypeId];
      if (!issueType) return;
      // the type still exists in the workspace, it is only disabled on this project
      set(
        this.issueTypeMap,
        [issueTypeId, "project_ids"],
        (issueType.project_ids ?? []).filter((id) => id !== projectId)
      );
    });
  };
}

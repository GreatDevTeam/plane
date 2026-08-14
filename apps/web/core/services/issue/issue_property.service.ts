/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { API_BASE_URL } from "@plane/constants";
import type {
  TBulkIssuePropertyValues,
  TIssueProperty,
  TIssuePropertyActivity,
  TIssuePropertyOption,
  TIssuePropertyPayload,
  TIssuePropertyOptionPayload,
  TIssuePropertyValues,
} from "@plane/types";
// services
import { APIService } from "@/services/api.service";

export class IssuePropertyService extends APIService {
  constructor() {
    super(API_BASE_URL);
  }

  /** The properties defined on one work item type. */
  async getIssueTypeProperties(workspaceSlug: string, issueTypeId: string): Promise<TIssueProperty[]> {
    return this.get(`/api/workspaces/${workspaceSlug}/issue-types/${issueTypeId}/issue-properties/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  /** The choices of one `OPTION` property. */
  async getIssuePropertyOptions(workspaceSlug: string, propertyId: string): Promise<TIssuePropertyOption[]> {
    return this.get(`/api/workspaces/${workspaceSlug}/issue-properties/${propertyId}/options/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async createIssueProperty(
    workspaceSlug: string,
    issueTypeId: string,
    data: TIssuePropertyPayload
  ): Promise<TIssueProperty> {
    return this.post(`/api/workspaces/${workspaceSlug}/issue-types/${issueTypeId}/issue-properties/`, data)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async updateIssueProperty(
    workspaceSlug: string,
    issueTypeId: string,
    propertyId: string,
    data: TIssuePropertyPayload
  ): Promise<TIssueProperty> {
    return this.patch(
      `/api/workspaces/${workspaceSlug}/issue-types/${issueTypeId}/issue-properties/${propertyId}/`,
      data
    )
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async deleteIssueProperty(workspaceSlug: string, issueTypeId: string, propertyId: string): Promise<void> {
    return this.delete(`/api/workspaces/${workspaceSlug}/issue-types/${issueTypeId}/issue-properties/${propertyId}/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async createIssuePropertyOption(
    workspaceSlug: string,
    propertyId: string,
    data: TIssuePropertyOptionPayload
  ): Promise<TIssuePropertyOption> {
    return this.post(`/api/workspaces/${workspaceSlug}/issue-properties/${propertyId}/options/`, data)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async updateIssuePropertyOption(
    workspaceSlug: string,
    propertyId: string,
    optionId: string,
    data: TIssuePropertyOptionPayload
  ): Promise<TIssuePropertyOption> {
    return this.patch(`/api/workspaces/${workspaceSlug}/issue-properties/${propertyId}/options/${optionId}/`, data)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async deleteIssuePropertyOption(workspaceSlug: string, propertyId: string, optionId: string): Promise<void> {
    return this.delete(`/api/workspaces/${workspaceSlug}/issue-properties/${propertyId}/options/${optionId}/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  /** The audit trail of one work item's property changes, for its activity feed. */
  async getIssuePropertyActivities(
    workspaceSlug: string,
    projectId: string,
    issueId: string,
    params?: { created_at__gt?: string }
  ): Promise<TIssuePropertyActivity[]> {
    return this.get(
      `/api/workspaces/${workspaceSlug}/projects/${projectId}/issues/${issueId}/issue-property-activities/`,
      { params }
    )
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async getIssuePropertyValues(
    workspaceSlug: string,
    projectId: string,
    issueId: string
  ): Promise<TIssuePropertyValues> {
    return this.get(`/api/workspaces/${workspaceSlug}/projects/${projectId}/issues/${issueId}/issue-property-values/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  /**
   * The values of a page of work items in one request. The board and spreadsheet
   * payloads have to stay cheap, so the values are fetched separately rather than
   * joined into the work item query; the endpoint is capped at 500 ids.
   */
  async getBulkIssuePropertyValues(
    workspaceSlug: string,
    projectId: string,
    issueIds: string[]
  ): Promise<TBulkIssuePropertyValues> {
    return this.get(`/api/workspaces/${workspaceSlug}/projects/${projectId}/issue-property-values/`, {
      params: { issue_ids: issueIds.join(",") },
    })
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  /**
   * The values of one workspace draft. A draft lives in its own table, so the create
   * modal saves against a separate endpoint; converting the draft carries them over.
   */
  async getDraftPropertyValues(workspaceSlug: string, draftId: string): Promise<TIssuePropertyValues> {
    return this.get(`/api/workspaces/${workspaceSlug}/draft-issues/${draftId}/issue-property-values/`)
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  async updateDraftPropertyValues(
    workspaceSlug: string,
    draftId: string,
    propertyValues: TIssuePropertyValues
  ): Promise<TIssuePropertyValues> {
    return this.post(`/api/workspaces/${workspaceSlug}/draft-issues/${draftId}/issue-property-values/`, {
      property_values: propertyValues,
    })
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }

  /**
   * Replaces the values of the submitted properties and returns the full map back.
   * Properties that are not in `propertyValues` are left alone, so a single edit in
   * the sidebar does not wipe the rest of the form. A rejected value comes back as a
   * `400` shaped `{ "<property_id>": "<message>" }`.
   */
  async updateIssuePropertyValues(
    workspaceSlug: string,
    projectId: string,
    issueId: string,
    propertyValues: TIssuePropertyValues
  ): Promise<TIssuePropertyValues> {
    return this.post(
      `/api/workspaces/${workspaceSlug}/projects/${projectId}/issues/${issueId}/issue-property-values/`,
      {
        property_values: propertyValues,
      }
    )
      .then((response) => response?.data)
      .catch((error) => {
        throw error?.response?.data;
      });
  }
}

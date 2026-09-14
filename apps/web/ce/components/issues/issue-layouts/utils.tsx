/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import type { FC } from "react";
import { useParams } from "next/navigation";
import { CalendarDays, LayersIcon, Paperclip } from "lucide-react";
// types
import { ISSUE_GROUP_BY_OPTIONS } from "@plane/constants";
import type { ISvgIcons } from "@plane/propel/icons";
import {
  LinkIcon,
  CycleIcon,
  StatePropertyIcon,
  ModuleIcon,
  MembersPropertyIcon,
  DueDatePropertyIcon,
  EstimatePropertyIcon,
  LabelPropertyIcon,
  PriorityPropertyIcon,
  StartDatePropertyIcon,
} from "@plane/propel/icons";
import type {
  IGroupByColumn,
  IIssueDisplayProperties,
  TGetColumns,
  TIssueGroupByOptions,
  TSpreadsheetColumn,
} from "@plane/types";
import { getWorkItemPropertyDisplayKey, getWorkItemPropertyIdFromDisplayKey } from "@plane/utils";
// components
import {
  SpreadsheetAssigneeColumn,
  SpreadsheetAttachmentColumn,
  SpreadsheetCreatedOnColumn,
  SpreadsheetDueDateColumn,
  SpreadsheetEstimateColumn,
  SpreadsheetLabelColumn,
  SpreadsheetModuleColumn,
  SpreadsheetCycleColumn,
  SpreadsheetLinkColumn,
  SpreadsheetPriorityColumn,
  SpreadsheetStartDateColumn,
  SpreadsheetStateColumn,
  SpreadsheetSubIssueColumn,
  SpreadsheetUpdatedOnColumn,
} from "@/components/issues/issue-layouts/spreadsheet/columns";
// store
import { store } from "@/lib/store-context";
// plane web imports
import { getWorkItemPropertySpreadsheetColumn } from "@/plane-web/components/issues/issue-layouts/property-values";
import { useIssueProperties } from "@/plane-web/hooks/store";
import { useProjectWorkItemProperties } from "@/plane-web/hooks/use-issue-properties";

/** A stable identity, so that a spreadsheet with no custom columns does not re-render. */
const EMPTY_ADDITIONAL_COLUMNS: (keyof IIssueDisplayProperties)[] = [];

export type TGetScopeMemberIdsResult = {
  memberIds: string[];
  includeNone: boolean;
};

export const getScopeMemberIds = ({ isWorkspaceLevel, projectId }: TGetColumns): TGetScopeMemberIdsResult => {
  // store values
  const { workspaceMemberIds } = store.memberRoot.workspace;
  const { projectMemberIds } = store.memberRoot.project;
  // derived values
  const memberIds = workspaceMemberIds;

  if (isWorkspaceLevel) {
    return { memberIds: memberIds ?? [], includeNone: true };
  }

  if (projectId || (projectMemberIds && projectMemberIds.length > 0)) {
    const { getProjectMemberIds } = store.memberRoot.project;
    const _projectMemberIds = projectId ? getProjectMemberIds(projectId, false) : projectMemberIds;
    return {
      memberIds: _projectMemberIds ?? [],
      includeNone: true,
    };
  }

  return { memberIds: [], includeNone: true };
};

export const getTeamProjectColumns = (): IGroupByColumn[] | undefined => undefined;

export const SpreadSheetPropertyIconMap: Record<string, FC<ISvgIcons>> = {
  MembersPropertyIcon: MembersPropertyIcon,
  CalenderDays: CalendarDays,
  DueDatePropertyIcon: DueDatePropertyIcon,
  EstimatePropertyIcon: EstimatePropertyIcon,
  LabelPropertyIcon: LabelPropertyIcon,
  ModuleIcon: ModuleIcon,
  ContrastIcon: CycleIcon,
  PriorityPropertyIcon: PriorityPropertyIcon,
  StartDatePropertyIcon: StartDatePropertyIcon,
  StatePropertyIcon: StatePropertyIcon,
  Link2: LinkIcon,
  Paperclip: Paperclip,
  LayersIcon: LayersIcon,
};

export const SPREADSHEET_COLUMNS: { [key in keyof IIssueDisplayProperties]: TSpreadsheetColumn } = {
  assignee: SpreadsheetAssigneeColumn,
  created_on: SpreadsheetCreatedOnColumn,
  due_date: SpreadsheetDueDateColumn,
  estimate: SpreadsheetEstimateColumn,
  labels: SpreadsheetLabelColumn,
  modules: SpreadsheetModuleColumn,
  cycle: SpreadsheetCycleColumn,
  link: SpreadsheetLinkColumn,
  priority: SpreadsheetPriorityColumn,
  start_date: SpreadsheetStartDateColumn,
  state: SpreadsheetStateColumn,
  sub_issue_count: SpreadsheetSubIssueColumn,
  updated_on: SpreadsheetUpdatedOnColumn,
  attachment_count: SpreadsheetAttachmentColumn,
};

/**
 * The column a display property key is rendered with. A `property_<uuid>` key stands for
 * a user defined property, whose column is built on the fly — the built-in ones are the
 * fixed set above.
 */
export const getSpreadsheetColumn = (property: keyof IIssueDisplayProperties): TSpreadsheetColumn | undefined => {
  const propertyId = getWorkItemPropertyIdFromDisplayKey(property);
  if (propertyId) return getWorkItemPropertySpreadsheetColumn(propertyId);
  return SPREADSHEET_COLUMNS[property];
};

/**
 * The custom property columns of a spreadsheet, in the order their properties are defined
 * in — appended after the built-in ones. A workspace level spreadsheet has no project to
 * read the definitions from, so it carries built-in columns only, and a toggle left over
 * from a property that has since been deleted is dropped.
 */
export const useAdditionalSpreadsheetColumns = (
  displayProperties: IIssueDisplayProperties,
  isWorkspaceLevel: boolean
): (keyof IIssueDisplayProperties)[] => {
  // router
  const { workspaceSlug, projectId } = useParams();
  // store hooks
  const { getProjectProperties } = useIssueProperties();
  // the toggles name properties without saying which type they belong to
  useProjectWorkItemProperties(
    isWorkspaceLevel ? null : workspaceSlug?.toString(),
    isWorkspaceLevel ? null : projectId?.toString()
  );

  if (isWorkspaceLevel || !projectId) return EMPTY_ADDITIONAL_COLUMNS;

  return getProjectProperties(projectId.toString())
    .filter((property) => !!displayProperties[getWorkItemPropertyDisplayKey(property.id)])
    .map((property) => getWorkItemPropertyDisplayKey(property.id));
};

export const useGroupByOptions = (
  options: TIssueGroupByOptions[]
): {
  key: TIssueGroupByOptions;
  titleTranslationKey: string;
}[] => {
  const groupByOptions = ISSUE_GROUP_BY_OPTIONS.filter((option) => options.includes(option.key));
  return groupByOptions;
};

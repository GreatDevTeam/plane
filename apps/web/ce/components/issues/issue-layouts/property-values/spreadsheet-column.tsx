/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useEffect } from "react";
import { observer } from "mobx-react";
import { useParams } from "next/navigation";
// plane imports
import type { TIssue, TIssuePropertyValue, TSpreadsheetColumn } from "@plane/types";
// plane web components
import { WorkItemPropertyValueEditor } from "@/plane-web/components/issues/issue-details/property-values";
// plane web hooks
import { useIssueProperties, useIssuePropertyValues } from "@/plane-web/hooks/store";

type TWorkItemPropertySpreadsheetColumnProps = {
  propertyId: string;
  issue: TIssue;
  disabled: boolean;
};

/**
 * One custom property of a work item as a spreadsheet cell. Unlike the card, a cell has
 * the room the phase 2 editors need, so a value is edited here the way the built-in
 * columns are — each edit is written straight through to the API.
 *
 * A work item whose type does not define the property is left empty: the column stands
 * for one property, and a spreadsheet can list work items of several types.
 */
const WorkItemPropertySpreadsheetColumn = observer(function WorkItemPropertySpreadsheetColumn(
  props: TWorkItemPropertySpreadsheetColumnProps
) {
  const { propertyId, issue, disabled } = props;
  // router
  const { workspaceSlug } = useParams();
  // store hooks
  const { getPropertyById, ensureIssueTypeProperties } = useIssueProperties();
  const { getPropertyValue, getPropertyError, updatePropertyValue, ensureWorkItemPropertyValues } =
    useIssuePropertyValues();
  // derived values
  const property = getPropertyById(propertyId);
  const belongsToWorkItem = !!property && property.issue_type_id === issue.type_id && property.is_active;
  const values = getPropertyValue(issue.id, propertyId);
  const error = getPropertyError(issue.id, propertyId);

  useEffect(() => {
    ensureIssueTypeProperties(workspaceSlug?.toString(), issue.type_id);
    ensureWorkItemPropertyValues(workspaceSlug?.toString(), issue.project_id, issue.id);
  }, [
    workspaceSlug,
    issue.type_id,
    issue.project_id,
    issue.id,
    ensureIssueTypeProperties,
    ensureWorkItemPropertyValues,
  ]);

  if (!belongsToWorkItem || !issue.project_id || !workspaceSlug) {
    return <div className="h-11 border-b-[0.5px] border-subtle" />;
  }

  const handleChange = (nextValues: TIssuePropertyValue[]) => {
    // a rejection is kept on the store and rendered by the cell it came from
    updatePropertyValue(workspaceSlug.toString(), issue.project_id!, issue.id, propertyId, nextValues).catch(() => {});
  };

  return (
    <div className="flex h-11 items-center border-b-[0.5px] border-subtle px-page-x">
      <WorkItemPropertyValueEditor
        property={property}
        values={values}
        disabled={disabled}
        hasError={Boolean(error)}
        onChange={handleChange}
        workspaceSlug={workspaceSlug.toString()}
        projectId={issue.project_id}
      />
    </div>
  );
});

/** Bound components, so that a re-render does not remount every cell of a column. */
const COLUMN_BY_PROPERTY_ID: Record<string, TSpreadsheetColumn> = {};

/**
 * The column a `property_<uuid>` display key stands for. The spreadsheet hands a column
 * only the work item it renders, so the property it belongs to is bound here.
 */
export const getWorkItemPropertySpreadsheetColumn = (propertyId: string): TSpreadsheetColumn => {
  if (!COLUMN_BY_PROPERTY_ID[propertyId]) {
    COLUMN_BY_PROPERTY_ID[propertyId] = function BoundWorkItemPropertyColumn({ issue, disabled }) {
      return <WorkItemPropertySpreadsheetColumn propertyId={propertyId} issue={issue} disabled={disabled} />;
    };
  }
  return COLUMN_BY_PROPERTY_ID[propertyId];
};

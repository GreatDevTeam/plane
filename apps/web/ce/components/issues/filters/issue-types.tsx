/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useMemo, useState } from "react";
import { sortBy } from "lodash-es";
import { observer } from "mobx-react";
import { useParams } from "next/navigation";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Logo } from "@plane/propel/emoji-icon-picker";
// components
import { FilterHeader, FilterOption } from "@/components/issues/issue-layouts/filters";
// plane web hooks
import { useIssueTypes } from "@/plane-web/hooks/store";

type Props = {
  appliedFilters: string[] | null;
  handleUpdate: (val: string) => void;
  searchQuery: string;
};

export const FilterIssueTypes = observer(function FilterIssueTypes(props: Props) {
  const { appliedFilters, handleUpdate, searchQuery } = props;
  // router params
  const { projectId } = useParams();
  // states
  const [itemsToRender, setItemsToRender] = useState(5);
  const [previewEnabled, setPreviewEnabled] = useState(true);
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getProjectIssueTypes } = useIssueTypes();
  // derived values
  const issueTypes = getProjectIssueTypes(projectId?.toString());
  const appliedFiltersCount = appliedFilters?.length ?? 0;

  const sortedOptions = useMemo(() => {
    const filteredOptions = issueTypes.filter((issueType) =>
      issueType.name.toLowerCase().includes(searchQuery.toLowerCase())
    );
    return sortBy(filteredOptions, [(issueType) => !(appliedFilters ?? []).includes(issueType.id)]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [issueTypes, searchQuery]);

  const handleViewToggle = () => {
    if (itemsToRender === sortedOptions.length) setItemsToRender(5);
    else setItemsToRender(sortedOptions.length);
  };

  // nothing worth filtering on when the project only has the default type
  if (issueTypes.length < 2) return null;

  return (
    <>
      <FilterHeader
        title={`${t("issue.display.properties.issue_type")}${appliedFiltersCount > 0 ? ` (${appliedFiltersCount})` : ""}`}
        isPreviewEnabled={previewEnabled}
        handleIsPreviewEnabled={() => setPreviewEnabled(!previewEnabled)}
      />
      {previewEnabled && (
        <div>
          {sortedOptions.length > 0 ? (
            <>
              {sortedOptions.slice(0, itemsToRender).map((issueType) => (
                <FilterOption
                  key={issueType.id}
                  isChecked={!!appliedFilters?.includes(issueType.id)}
                  onClick={() => handleUpdate(issueType.id)}
                  icon={<Logo logo={issueType.logo_props} size={14} type="lucide" />}
                  title={issueType.name}
                />
              ))}
              {sortedOptions.length > 5 && (
                <button
                  type="button"
                  className="ml-8 text-11 font-medium text-accent-primary"
                  onClick={handleViewToggle}
                >
                  {itemsToRender === sortedOptions.length ? "View less" : "View all"}
                </button>
              )}
            </>
          ) : (
            <p className="text-11 text-placeholder italic">{t("no_matching_results")}</p>
          )}
        </div>
      )}
    </>
  );
});

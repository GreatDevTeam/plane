/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useState } from "react";
import { observer } from "mobx-react";
import { useParams } from "next/navigation";
import useSWR from "swr";
// plane imports
import { EUserPermissions, EUserPermissionsLevel } from "@plane/constants";
import { useTranslation } from "@plane/i18n";
import { Button } from "@plane/propel/button";
import { Loader } from "@plane/ui";
// components
import { SettingsHeading } from "@/components/settings/heading";
// hooks
import { useUserPermissions } from "@/hooks/store/user";
// plane web hooks
import { useIssueTypes } from "@/plane-web/hooks/store";
// local imports
import { WorkItemTypeForm } from "./type-form";
import { WorkItemTypeItem } from "./type-item";

/**
 * Project settings → Work item types. Defines the types the project's work items can
 * have and, per type, the custom fields they carry.
 */
export const ProjectWorkItemTypesRoot = observer(function ProjectWorkItemTypesRoot() {
  // router
  const { workspaceSlug, projectId } = useParams();
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getAllProjectIssueTypes, fetchProjectIssueTypes, createIssueType } = useIssueTypes();
  const { allowPermissions } = useUserPermissions();
  // states
  const [isCreating, setIsCreating] = useState(false);
  // derived values
  const isEditable = allowPermissions([EUserPermissions.ADMIN], EUserPermissionsLevel.PROJECT);
  const issueTypes = getAllProjectIssueTypes(projectId?.toString());

  const { isLoading } = useSWR(
    workspaceSlug && projectId ? `PROJECT_WORK_ITEM_TYPES_${workspaceSlug}_${projectId}` : null,
    workspaceSlug && projectId ? () => fetchProjectIssueTypes(workspaceSlug.toString(), projectId.toString()) : null,
    { revalidateIfStale: false, revalidateOnFocus: false }
  );

  return (
    <>
      <SettingsHeading
        title={t("project_settings.work_item_types.heading")}
        description={t("project_settings.work_item_types.description")}
        control={
          isEditable &&
          !isCreating && (
            <Button variant="primary" size="lg" onClick={() => setIsCreating(true)}>
              {t("project_settings.work_item_types.add_type")}
            </Button>
          )
        }
      />
      <div className="mt-6 flex w-full flex-col gap-3">
        {isCreating && workspaceSlug && projectId && (
          <WorkItemTypeForm
            onSubmit={(data) => createIssueType(workspaceSlug.toString(), projectId.toString(), data)}
            onClose={() => setIsCreating(false)}
          />
        )}
        {isLoading && issueTypes.length === 0 ? (
          <Loader className="flex flex-col gap-3">
            <Loader.Item height="44px" />
            <Loader.Item height="44px" />
            <Loader.Item height="44px" />
          </Loader>
        ) : issueTypes.length === 0 && !isCreating ? (
          <div className="py-10 text-center">
            <p className="text-16 font-medium text-primary">{t("project_settings.work_item_types.no_types")}</p>
            <p className="mt-1 text-13 text-secondary">{t("project_settings.work_item_types.no_types_description")}</p>
          </div>
        ) : (
          issueTypes.map((issueType) => (
            <WorkItemTypeItem
              key={issueType.id}
              workspaceSlug={workspaceSlug?.toString() ?? ""}
              projectId={projectId?.toString() ?? ""}
              issueType={issueType}
              disabled={!isEditable}
            />
          ))
        )}
      </div>
    </>
  );
});

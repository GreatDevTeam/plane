/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useEffect, useRef } from "react";
import { observer } from "mobx-react";
import { useFormContext } from "react-hook-form";
import { Info } from "lucide-react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Tooltip } from "@plane/propel/tooltip";
import type { TIssue, TIssueProperty, TIssuePropertyValue } from "@plane/types";
// components
import { SidebarPropertyListItem } from "@/components/common/layout/sidebar/property-list-item";
import {
  WorkItemPropertyValueEditor,
  workItemPropertyIcon,
} from "@/plane-web/components/issues/issue-details/property-values";
// hooks
import { useIssueModal } from "@/hooks/context/use-issue-modal";
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web imports
import { useIssueProperties, useIssuePropertyValues } from "@/plane-web/hooks/store";
import { useWorkItemModalProperties } from "@/plane-web/hooks/use-issue-properties";

export type TWorkItemModalAdditionalPropertiesProps = {
  isDraft?: boolean;
  projectId: string | null;
  workItemId: string | undefined;
  workspaceSlug: string;
};

/**
 * The custom properties of the work item being created or updated, rendered by the
 * modal below the description.
 *
 * Unlike the detail sidebar nothing is written through: the values live in the modal's
 * form state until the work item exists, and the provider saves them once it does.
 */
export const WorkItemModalAdditionalProperties = observer(function WorkItemModalAdditionalProperties(
  props: TWorkItemModalAdditionalPropertiesProps
) {
  const { isDraft = false, projectId, workItemId, workspaceSlug } = props;
  // form context — the modal wraps the whole form, the picked type drives the field set
  const { watch } = useFormContext<TIssue>();
  const workItemTypeId = watch("type_id");
  // context hooks
  const { issuePropertyValues, setIssuePropertyValues, issuePropertyValueErrors, setIssuePropertyValueErrors } =
    useIssueModal();
  // store hooks
  const { getActiveIssueTypeProperties } = useIssueProperties();
  const { getWorkItemPropertyValues } = useIssuePropertyValues();
  const { arePropertiesReady, areSavedValuesReady } = useWorkItemModalProperties(
    workspaceSlug,
    projectId,
    workItemId,
    workItemTypeId,
    isDraft
  );
  // derived values
  const properties = getActiveIssueTypeProperties(workItemTypeId);

  // What the form was last seeded for. Picking another type swaps the whole field set,
  // so the values of the previous one are dropped rather than carried over.
  const seededFor = useRef<string | null>(null);
  const seedKey = `${workItemTypeId ?? ""}:${workItemId ?? ""}`;

  useEffect(() => {
    if (seededFor.current === seedKey) return;
    // clear straight away, so a submit landing before the new type's fields arrive
    // cannot save the previous type's values
    setIssuePropertyValues({});
    setIssuePropertyValueErrors({});

    if (!workItemTypeId) {
      seededFor.current = seedKey;
      return;
    }
    if (!arePropertiesReady || !areSavedValuesReady) return;

    seededFor.current = seedKey;
    const savedValues = getWorkItemPropertyValues(workItemId);
    setIssuePropertyValues(
      Object.fromEntries(
        properties.map((property) => [property.id, savedValues?.[property.id] ?? property.default_value ?? []])
      )
    );
    // `properties` and the saved values are read off the store the readiness flags stand for
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedKey, arePropertiesReady, areSavedValuesReady]);

  const handleChange = (property: TIssueProperty, values: TIssuePropertyValue[]) => {
    setIssuePropertyValues((previousValues) => ({ ...previousValues, [property.id]: values }));
    setIssuePropertyValueErrors((previousErrors) => {
      if (!previousErrors[property.id]) return previousErrors;
      const { [property.id]: _cleared, ...remainingErrors } = previousErrors;
      return remainingErrors;
    });
  };

  if (!projectId || properties.length === 0) return null;

  return (
    <div className="space-y-2 px-5">
      {properties.map((property) => (
        <WorkItemModalProperty
          key={property.id}
          property={property}
          values={issuePropertyValues[property.id] ?? []}
          error={issuePropertyValueErrors[property.id]}
          workspaceSlug={workspaceSlug}
          projectId={projectId}
          onChange={handleChange}
        />
      ))}
    </div>
  );
});

type TWorkItemModalPropertyProps = {
  property: TIssueProperty;
  values: TIssuePropertyValue[];
  error: string | undefined;
  workspaceSlug: string;
  projectId: string;
  onChange: (property: TIssueProperty, values: TIssuePropertyValue[]) => void;
};

const WorkItemModalProperty = observer(function WorkItemModalProperty(props: TWorkItemModalPropertyProps) {
  const { property, values, error, workspaceSlug, projectId, onChange } = props;
  // plane hooks
  const { t } = useTranslation();
  const { isMobile } = usePlatformOS();

  return (
    <SidebarPropertyListItem
      icon={workItemPropertyIcon(property)}
      label={property.display_name}
      appendElement={
        <>
          {property.is_required && (
            <span className="text-danger-primary" aria-label={t("work_item_properties.required")}>
              *
            </span>
          )}
          {property.description && (
            <Tooltip tooltipContent={property.description} isMobile={isMobile}>
              <Info className="size-3 shrink-0 text-tertiary" />
            </Tooltip>
          )}
        </>
      }
      childrenClassName="flex-col items-stretch"
    >
      <WorkItemPropertyValueEditor
        property={property}
        values={values}
        disabled={false}
        hasError={Boolean(error)}
        onChange={(nextValues) => onChange(property, nextValues)}
        workspaceSlug={workspaceSlug}
        projectId={projectId}
      />
      {error && <p className="px-2 text-caption-sm-regular text-danger-primary">{error}</p>}
    </SidebarPropertyListItem>
  );
});

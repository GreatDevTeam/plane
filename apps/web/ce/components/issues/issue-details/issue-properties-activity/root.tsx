/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { observer } from "mobx-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ListFilter } from "lucide-react";
// plane imports
import { Tooltip } from "@plane/propel/tooltip";
import { calculateTimeAgo, renderFormattedDate, renderFormattedTime } from "@plane/utils";
// hooks
import { useIssueDetail } from "@/hooks/store/use-issue-detail";
import { useMember } from "@/hooks/store/use-member";
import { usePlatformOS } from "@/hooks/use-platform-os";
// plane web hooks
import { useIssueProperties } from "@/plane-web/hooks/store";

type TIssueAdditionalPropertiesActivity = {
  activityId: string;
  ends: "top" | "bottom" | undefined;
};

/** One value of a custom field as the feed shows it. */
function PropertyValue({ value }: { value: string | null }) {
  return <span className="font-medium text-primary">{value || "none"}</span>;
}

/**
 * One custom field change in the work item activity feed.
 *
 * The values are the text the endpoint recorded when the change was made — an option, a
 * member and a related work item by name rather than by id — so the feed keeps reading
 * right after an option is renamed or a value is deleted.
 */
export const IssueAdditionalPropertiesActivity = observer(function IssueAdditionalPropertiesActivity(
  props: TIssueAdditionalPropertiesActivity
) {
  const { activityId, ends } = props;
  // router
  const { workspaceSlug } = useParams();
  // store hooks
  const {
    activity: { getPropertyActivityById },
  } = useIssueDetail();
  const { getPropertyById } = useIssueProperties();
  const { getUserDetails } = useMember();
  const { isMobile } = usePlatformOS();
  // derived values
  const activity = getPropertyActivityById(activityId);
  const property = getPropertyById(activity?.property_id);
  const actor = activity?.actor ? getUserDetails(activity.actor) : undefined;

  if (!activity) return <></>;

  return (
    <div
      className={`relative flex items-center gap-3 text-caption-sm-regular ${
        ends === "top" ? `pb-2` : ends === "bottom" ? `pt-2` : `py-2`
      }`}
    >
      <div className="absolute top-0 bottom-0 left-[13px] w-px bg-layer-3" aria-hidden />
      <div className="z-[4] flex h-7 w-7 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg border border-subtle bg-layer-2 text-secondary shadow-raised-100">
        <ListFilter className="h-3.5 w-3.5" />
      </div>
      <div className="w-full truncate text-secondary">
        {actor ? (
          <Link href={`/${workspaceSlug}/profile/${actor.id}`} className="font-medium text-primary hover:underline">
            {actor.display_name}
          </Link>
        ) : (
          <span className="font-medium text-primary">Plane</span>
        )}
        <span>
          {" "}
          {activity.action === "created" ? "set" : activity.action === "deleted" ? "cleared" : "changed"}{" "}
          <span className="font-medium text-primary">{property?.display_name ?? "a custom field"}</span>
          {activity.action === "created" && (
            <>
              {" to "}
              <PropertyValue value={activity.new_value} />
            </>
          )}
          {activity.action === "updated" && (
            <>
              {" from "}
              <PropertyValue value={activity.old_value} />
              {" to "}
              <PropertyValue value={activity.new_value} />
            </>
          )}
          {activity.action === "deleted" && activity.old_value && (
            <>
              {", was "}
              <PropertyValue value={activity.old_value} />
            </>
          )}
        </span>
        <span>
          <Tooltip
            isMobile={isMobile}
            tooltipContent={`${renderFormattedDate(activity.created_at)}, ${renderFormattedTime(activity.created_at)}`}
          >
            <span className="whitespace-nowrap text-tertiary"> {calculateTimeAgo(activity.created_at)}</span>
          </Tooltip>
        </span>
      </div>
    </div>
  );
});

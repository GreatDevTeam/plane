/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

// store
import { CoreRootStore } from "@/store/root.store";
import type { IIssuePropertiesStore } from "./issue-properties";
import { IssuePropertiesStore } from "./issue-properties";
import type { IIssuePropertyValuesStore } from "./issue-property-values";
import { IssuePropertyValuesStore } from "./issue-property-values";
import type { IIssueTypesStore } from "./issue-types";
import { IssueTypesStore } from "./issue-types";
import type { ITimelineStore } from "./timeline";
import { TimeLineStore } from "./timeline";

export class RootStore extends CoreRootStore {
  timelineStore: ITimelineStore;
  issueTypes: IIssueTypesStore;
  issueProperties: IIssuePropertiesStore;
  issuePropertyValues: IIssuePropertyValuesStore;

  constructor() {
    super();

    this.timelineStore = new TimeLineStore(this);
    this.issueTypes = new IssueTypesStore(this);
    this.issueProperties = new IssuePropertiesStore(this);
    this.issuePropertyValues = new IssuePropertyValuesStore(this);
  }
}

/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useEffect, useRef } from "react";

const AUTO_REFRESH_INTERVAL = 30000;

/**
 * Polls the board for fresh issue data on an interval without triggering a full reload.
 * Uses the "mutation" loader so the board stays visible during the fetch.
 * Skips the poll when another fetch is already in flight (indicated by shouldSkip).
 */
const useAutoRefreshIssues = (refreshFn: () => Promise<void>, shouldSkip: () => boolean) => {
  const refreshFnRef = useRef(refreshFn);
  const shouldSkipRef = useRef(shouldSkip);

  useEffect(() => {
    refreshFnRef.current = refreshFn;
  }, [refreshFn]);

  useEffect(() => {
    shouldSkipRef.current = shouldSkip;
  }, [shouldSkip]);

  useEffect(() => {
    const intervalId = setInterval(() => {
      if (shouldSkipRef.current()) return;
      refreshFnRef.current().catch(() => {});
    }, AUTO_REFRESH_INTERVAL);

    return () => clearInterval(intervalId);
  }, []);
};

export default useAutoRefreshIssues;

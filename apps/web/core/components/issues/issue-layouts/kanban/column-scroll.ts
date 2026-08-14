/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

/**
 * Vertical scroll preservation for the kanban state columns across a background refresh.
 *
 * A refresh briefly clears `groupedIssueIds`, so every column and every card unmounts and
 * mounts again. A raw `scrollTop` restore then lands on the wrong work item, because
 * `RenderIfVisible` re-mounts every card past the tenth as a 100px placeholder and only
 * measures its real height a few frames later. Each column is therefore anchored to a single
 * card — the one the user is working with, or the one at the top of the column — and the
 * anchor is re-aligned until the column stops moving.
 */

/** spread onto the scrollable state column div so the root can find every column without coupling to class names */
export const KANBAN_COLUMN_PROPS = { "data-kanban-column": "" };

const KANBAN_COLUMN_SELECTOR = "[data-kanban-column]";
const ISSUE_BLOCK_ID_PREFIX = "issue-";
const ISSUE_BLOCK_SELECTOR = `[id^="${ISSUE_BLOCK_ID_PREFIX}"]`;

/** how long the anchor keeps being re-aligned while the remounted cards settle to their real heights */
const SETTLE_DURATION = 1000;
/** sub-pixel drift is not worth a scroll write */
const MIN_CORRECTION = 1;

export type TKanbanColumnScroll = {
  /** raw scroll offset, used as a fallback when the anchored work item is gone after the refresh */
  scrollTop: number;
  /** the work item the column is anchored to */
  anchorIssueId: string | undefined;
  /** distance between the top of the column and the top of the anchor card */
  anchorOffset: number;
};

const getColumns = (container: HTMLElement) =>
  Array.from(container.querySelectorAll<HTMLElement>(KANBAN_COLUMN_SELECTOR));

const getIssueBlock = (column: HTMLElement, issueId: string) =>
  column.querySelector<HTMLElement>(`[id="${ISSUE_BLOCK_ID_PREFIX}${issueId}"]`);

const getIssueIdFromBlock = (block: HTMLElement) => block.id.slice(ISSUE_BLOCK_ID_PREFIX.length) || undefined;

/** distance from the top of the column's viewport down to the top of the card */
const getOffsetWithinColumn = (column: HTMLElement, block: HTMLElement) =>
  block.getBoundingClientRect().top - column.getBoundingClientRect().top;

const isBlockOnScreen = (column: HTMLElement, block: HTMLElement) => {
  const offset = getOffsetWithinColumn(column, block);
  return offset < column.clientHeight && offset + block.offsetHeight > 0;
};

/** the card the user is working with: the one holding keyboard focus, else the one open in the peek view */
const getFocusedBlock = (column: HTMLElement, peekIssueId: string | undefined) => {
  const activeElement = document.activeElement;
  if (activeElement instanceof HTMLElement && column.contains(activeElement)) {
    const focusedBlock = activeElement.closest<HTMLElement>(ISSUE_BLOCK_SELECTOR);
    if (focusedBlock) return focusedBlock;
  }
  return peekIssueId ? getIssueBlock(column, peekIssueId) : null;
};

/** the first card still on screen — the one the user reads as "first in the column" */
const getTopVisibleBlock = (column: HTMLElement) =>
  Array.from(column.querySelectorAll<HTMLElement>(ISSUE_BLOCK_SELECTOR)).find(
    (block) => getOffsetWithinColumn(column, block) + block.offsetHeight > 0
  ) ?? null;

/**
 * Snapshots the vertical scroll of every column, anchored to the work item the user is looking at.
 * @param container the horizontally scrollable board element
 * @param peekIssueId the work item open in the peek view, if any
 */
export const captureKanbanColumnScrolls = (
  container: HTMLElement | null,
  peekIssueId?: string
): Map<string, TKanbanColumnScroll> => {
  const snapshots = new Map<string, TKanbanColumnScroll>();
  if (!container) return snapshots;

  getColumns(container).forEach((column) => {
    // an unscrolled column has nothing to restore — it already renders its first item first
    if (!column.id || column.scrollTop <= 0) return;

    const focusedBlock = getFocusedBlock(column, peekIssueId);
    // the focused card only makes sense as an anchor while it is on screen: anchoring to one the
    // user has since scrolled away from would drag the column back to it
    const anchor = focusedBlock && isBlockOnScreen(column, focusedBlock) ? focusedBlock : getTopVisibleBlock(column);

    snapshots.set(column.id, {
      scrollTop: column.scrollTop,
      anchorIssueId: anchor ? getIssueIdFromBlock(anchor) : undefined,
      anchorOffset: anchor ? getOffsetWithinColumn(column, anchor) : 0,
    });
  });

  return snapshots;
};

/** Scrolls one column back onto its anchor. Returns false once there is nothing left to track. */
const alignColumn = (column: HTMLElement, snapshot: TKanbanColumnScroll) => {
  const anchor = snapshot.anchorIssueId ? getIssueBlock(column, snapshot.anchorIssueId) : null;
  if (!anchor) {
    column.scrollTop = snapshot.scrollTop;
    return false;
  }

  const correction = getOffsetWithinColumn(column, anchor) - snapshot.anchorOffset;
  if (Math.abs(correction) >= MIN_CORRECTION) column.scrollTop += correction;
  return true;
};

/**
 * Restores the snapshotted column scrolls, re-aligning each anchor every frame until the
 * remounted cards stop resizing (or the user scrolls, whichever comes first).
 * @returns a function that cancels the pending re-alignment
 */
export const restoreKanbanColumnScrolls = (
  container: HTMLElement | null,
  snapshots: Map<string, TKanbanColumnScroll>
): (() => void) => {
  if (!container || snapshots.size === 0) return () => {};

  const pending = new Map(snapshots);
  const deadline = Date.now() + SETTLE_DURATION;
  let frameId: number | null = null;

  const stop = () => {
    if (frameId !== null) cancelAnimationFrame(frameId);
    frameId = null;
    container.removeEventListener("wheel", stop);
    container.removeEventListener("touchmove", stop);
    container.removeEventListener("pointerdown", stop);
    window.removeEventListener("keydown", stop);
  };

  const align = () => {
    frameId = null;
    pending.forEach((snapshot, columnId) => {
      const column = document.getElementById(columnId);
      // a column that has not been rendered back yet is retried on the next frame
      if (!column || !container.contains(column)) return;
      if (!alignColumn(column, snapshot)) pending.delete(columnId);
    });

    if (pending.size === 0 || Date.now() > deadline) {
      stop();
      return;
    }
    frameId = requestAnimationFrame(align);
  };

  // the user owns the scroll position the moment they touch it — keyboard included, since
  // tabbing between cards scrolls the column too
  container.addEventListener("wheel", stop, { passive: true });
  container.addEventListener("touchmove", stop, { passive: true });
  container.addEventListener("pointerdown", stop, { passive: true });
  window.addEventListener("keydown", stop, { passive: true });

  align();
  return stop;
};

/**
 * "Load more" history appended under a polled first page (cron runs, w4/230;
 * deploys, w4/232).
 *
 * The first page is a polled Apollo query; older pages are read imperatively
 * past the last visible id and held in React state, because a poll rewrites
 * the first-page cache entry and would truncate a merged one. A new arrival
 * shifts the first page, so the held tail can stop joining it: head
 * [9…5] + tail [4…1] becomes head [10…6], and 5 belongs to neither. Rendering
 * the tail anyway hides a row mid-history behind an exhausted Load more.
 *
 * So a tail records the head boundary it hangs off (`anchorId`, the head's
 * last id when its first page was read) and joins only a head that still ends
 * there. The stored state itself, not just the render, follows that rule: a
 * Load more page starts a fresh tail when read from the head's end, extends
 * the tail only when read from that exact tail's end, and is otherwise a late
 * answer for a context that no longer exists and is discarded.
 */
export interface AppendedPages<Row> {
  /** The first-page query the tail belongs to (service, filters). */
  key: string;
  /** The head's last id when the tail's first page was read. */
  anchorId: string;
  rows: Row[];
  /** Size of the most recently appended page (null ⇒ none appended yet). */
  lastPageSize: number | null;
}

export const NO_APPENDED: AppendedPages<never> = {
  key: "",
  anchorId: "",
  rows: [],
  lastPageSize: null,
};

/** Where a Load more read was taken from. */
export interface AppendRequest {
  key: string;
  /** The tail's anchor, or the head's last id when no tail is joined. */
  anchorId: string;
  /** The last visible id the page was read past. */
  cursor: string;
}

/**
 * The visible rows — head plus the stored tail when it still joins — the tail
 * in effect (NO_APPENDED when it doesn't join), whether an older page may
 * exist, and the request a Load more from here would make.
 */
export function joinAppendedPages<Row extends { id: string }>(
  head: Row[],
  appended: AppendedPages<Row>,
  key: string,
  pageSize: number,
): {
  rows: Row[];
  tail: AppendedPages<Row>;
  hasMore: boolean;
  next: AppendRequest | null;
} {
  const headIds = new Set(head.map((row) => row.id));
  const joins =
    appended.key === key &&
    appended.anchorId !== "" &&
    head.at(-1)?.id === appended.anchorId &&
    !appended.rows.some((row) => headIds.has(row.id));
  const tail = joins ? appended : NO_APPENDED;
  const rows = [...head, ...tail.rows];
  const cursor = rows.at(-1)?.id;
  return {
    rows,
    tail,
    // Until a page is appended the head decides; after, the last page does.
    hasMore:
      tail.lastPageSize === null
        ? head.length === pageSize
        : tail.lastPageSize === pageSize,
    next: cursor
      ? { key, anchorId: tail.anchorId || (head.at(-1)?.id ?? ""), cursor }
      : null,
  };
}

/** The stored tail after a successful Load more read of `page`. */
export function appendPage<Row extends { id: string }>(
  prev: AppendedPages<Row>,
  request: AppendRequest,
  page: Row[],
): AppendedPages<Row> {
  if (request.cursor === request.anchorId) {
    return {
      key: request.key,
      anchorId: request.anchorId,
      rows: page,
      lastPageSize: page.length,
    };
  }
  const continues =
    prev.key === request.key &&
    prev.anchorId === request.anchorId &&
    prev.rows.at(-1)?.id === request.cursor;
  if (!continues) return prev;
  const seen = new Set(prev.rows.map((row) => row.id));
  return {
    ...prev,
    rows: [...prev.rows, ...page.filter((row) => !seen.has(row.id))],
    lastPageSize: page.length,
  };
}

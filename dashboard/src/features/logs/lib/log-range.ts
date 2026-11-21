import {
  CLEARED_RANGE_SEARCH,
  isCustomRange,
  parseCustomRange,
  parseRangePreset,
  parseRangeSearch,
  rangeToSearch,
  type CustomRange,
  type RangePreset,
  type RangeSearch,
  type RangeSelection,
} from "@/features/metrics/lib/range";

// A log explorer's time range, as Render's is (w3/m537): Live tail, a
// relative preset or a custom window. Render's explorer keeps one
// `timeRange` whose first value is `live`, and tails exactly while it is
// (`liveMode: timeRange === "live"`); a preset or a custom window never
// tails. Metrics' RangeSelection and its URL keys stay as they are: Live
// tail is a log explorer's range only.

/** Render's Live tail range: its `timeRange` value and URL spelling. */
export interface LiveTailRange {
  id: "live";
}

export const LIVE_TAIL: LiveTailRange = { id: "live" };

/** A log explorer's range: Live tail, a preset or a custom window. */
export type LogRange = LiveTailRange | RangeSelection;

export type LogRangeID = LogRange["id"];

export function isLiveTail(range: LogRange): range is LiveTailRange {
  return range.id === LIVE_TAIL.id;
}

export function isCustomLogRange(range: LogRange): range is CustomRange {
  return !isLiveTail(range) && isCustomRange(range);
}

/** The newest instant a range's lines may carry: a custom window's end. */
export function logRangeEndMs(range: LogRange): number {
  return range.id === "custom" ? Date.parse(range.endTime) : Infinity;
}

/** A relative preset: the ranges Render offers New logs in Live Tail on. */
export function isPresetRange(range: LogRange): range is RangePreset {
  return !isLiveTail(range) && !isCustomRange(range);
}

/** The range an explorer opens on when its URL names none, and the one it
 *  reads where bex's tail cannot run: Last hour, Render's default for
 *  runtime logs (render.com/docs/logging, "The default displayed range is
 *  Last hour"). */
export const DEFAULT_LOG_RANGE = parseRangePreset("1h")!;

/** What Live tail shows before new lines arrive: Render's reads the last 4
 *  hours (`live` → 4 in the explorer's range-to-hours table), older pages
 *  loading on scroll, and bex's does the same. */
export const LIVE_TAIL_HISTORY = parseRangePreset("4h")!;

/** The window an explorer's history reads for its range. */
export function historyRange(range: LogRange): RangeSelection {
  return isLiveTail(range) ? LIVE_TAIL_HISTORY : range;
}

/** The shared range keys, with Live tail as a `range` value. */
export interface LogRangeSearch {
  range?: LogRangeID;
  rangeStart?: string;
  rangeEnd?: string;
}

/** Validates the range keys off a raw URL search: Metrics' keys plus
 *  `range=live`, which Metrics' own parser refuses. */
export function parseLogRangeSearch(
  search: Record<string, unknown>,
): LogRangeSearch {
  return search.range === LIVE_TAIL.id
    ? { ...CLEARED_RANGE_SEARCH, range: LIVE_TAIL.id }
    : parseRangeSearch(search);
}

/** The range validated keys name, else `fallback`. */
export function logRangeFromSearch(
  search: LogRangeSearch,
  fallback: LogRange,
): LogRange {
  if (search.range === LIVE_TAIL.id) return LIVE_TAIL;
  if (search.range === "custom") {
    return parseCustomRange(search.rangeStart, search.rangeEnd) ?? fallback;
  }
  return parseRangePreset(search.range) ?? fallback;
}

/** The keys as a Metrics range reads them (a datastore's Metrics tab shares
 *  its Logs tab's keys): Live tail is a log explorer's range only. */
export function metricsRangeSearch(search: LogRangeSearch): RangeSearch {
  const { range, rangeStart, rangeEnd } = search;
  return range === LIVE_TAIL.id
    ? { ...CLEARED_RANGE_SEARCH }
    : { range, rangeStart, rangeEnd };
}

/** A range as the URL's keys, each written so a stale custom bound clears. */
export function logRangeToSearch(range: LogRange): LogRangeSearch {
  return isLiveTail(range)
    ? { ...CLEARED_RANGE_SEARCH, range: LIVE_TAIL.id }
    : rangeToSearch(range);
}

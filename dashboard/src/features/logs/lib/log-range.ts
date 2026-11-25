import {
  CLEARED_RANGE_SEARCH,
  isCustomRange,
  parseCustomRange,
  parseRangePreset,
  parseRangeSearch,
  rangeToSearch,
  type CustomRange,
  type RangePreset,
  type RangePresetID,
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

/** The presets a log explorer lists between Live tail and Custom, and a log
 *  view's Time Range offers (w3/100): Render's `logTimeRangeIntervals`
 *  after `live`. Metrics keeps its own ladder, which adds 30m and 12h. */
export const LOG_RANGE_PRESETS: RangePreset[] = (
  ["1h", "4h", "24h", "2d", "7d", "14d", "30d"] as const
).map((id) => parseRangePreset(id)!);

// Metrics' presets Render's Logs never list, each read as the log range
// nearest it on the ladder's ratio scale: Last hour is twice 30 minutes, and
// Last 24 hours twice 12 hours where Last 4 hours is a third of it.
const METRICS_ONLY_RANGES: Partial<Record<RangePresetID, RangePresetID>> = {
  "30m": "1h",
  "12h": "24h",
};

/** A range as a log explorer reads it: Metrics' 30m and 12h, from an old
 *  link or a view saved with one, open on the nearest log range. */
export function toLogRange(range: LogRange): LogRange {
  const nearest = isPresetRange(range) && METRICS_ONLY_RANGES[range.id];
  return nearest ? parseRangePreset(nearest)! : range;
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

/** The range validated keys name, else `fallback`, as a log explorer reads
 *  it (`toLogRange`). The keys keep Metrics' presets: a Key Value's Metrics
 *  tab shares them. */
export function logRangeFromSearch(
  search: LogRangeSearch,
  fallback: LogRange,
): LogRange {
  if (search.range === LIVE_TAIL.id) return LIVE_TAIL;
  const range =
    search.range === "custom"
      ? parseCustomRange(search.rangeStart, search.rangeEnd)
      : parseRangePreset(search.range);
  return toLogRange(range ?? fallback);
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

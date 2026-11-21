import {
  parseCustomRange,
  parseRangePreset,
  type RangePreset,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { SavedSearchType } from "@/graphql/definitions";
import type { LogLine } from "../types";
import { documentTimeRange } from "./view-filters";
import {
  LIVE_TAIL,
  isCustomLogRange,
  type LogRange,
  type LogRangeID,
} from "./log-range";
import {
  logContextSearch,
  parseLogRangeAlias,
  searchHref,
  str,
} from "./log-search";

/**
 * A project observability view's URL (w3/m505), in Render's keys:
 * `/project/{prj}/observability/logs/application?savedSearchId=svs-…&r=1h`.
 * `savedSearchId` names the view and `r` the range, Render's `r=live` for
 * Live tail (w3/m537); a custom range keeps the
 * Logs tab's `rangeStart`/`rangeEnd` beside `r=custom`. Every key comes back
 * (undefined when absent or rejected) because the router merges the validated
 * search over the raw one, and a key it omits lingers (w7/m42).
 */
export interface ProjectLogSearch {
  savedSearchId?: string;
  r?: LogRangeID;
  rangeStart?: string;
  rangeEnd?: string;
  /** The line a View in context link marks (w3/m528): its key. */
  contextLine?: string;
}

export function parseProjectLogSearch(
  search: Record<string, unknown>,
): ProjectLogSearch {
  const out: ProjectLogSearch = {
    savedSearchId: str(search.savedSearchId),
    r: undefined,
    rangeStart: undefined,
    rangeEnd: undefined,
    contextLine: str(search.contextLine),
  };
  if (search.r === "custom") {
    const start = str(search.rangeStart);
    const end = str(search.rangeEnd);
    if (start && end && parseCustomRange(start, end)) {
      out.r = "custom";
      out.rangeStart = start;
      out.rangeEnd = end;
    }
    return out;
  }
  // Render's own `r` tokens without a bex preset map onto the nearest one,
  // as the Logs tab's alias does.
  out.r = parseLogRangeAlias(search.r);
  return out;
}

/** A project Service Metrics view's URL (w3/m508): a log view's keys, with
 *  Live tail refused, as Metrics has none. */
export function parseProjectMetricsSearch(
  search: Record<string, unknown>,
): ProjectLogSearch {
  const out = parseProjectLogSearch(search);
  return out.r === LIVE_TAIL.id ? { ...out, r: undefined } : out;
}

/** The preset or custom range the URL names, else `fallback`: what a
 *  project Service Metrics view reads. */
export function projectMetricsRange(
  search: ProjectLogSearch,
  fallback: RangeSelection,
): RangeSelection {
  if (search.r === "custom" && search.rangeStart && search.rangeEnd) {
    const custom = parseCustomRange(search.rangeStart, search.rangeEnd);
    if (custom) return custom;
  }
  return parseRangePreset(search.r) ?? fallback;
}

/** The range the URL names, Live tail among them, else the view's own
 *  default (its saved time range, else Last hour). */
export function projectLogRange(
  search: ProjectLogSearch,
  fallback: RangeSelection,
): LogRange {
  return search.r === LIVE_TAIL.id
    ? LIVE_TAIL
    : projectMetricsRange(search, fallback);
}

/** A range as the URL's keys, every one written so a stale bound clears,
 *  and a View in context mark with it (w3/m528). */
export function projectLogRangeSearch(
  range: LogRange,
): Pick<ProjectLogSearch, "r" | "rangeStart" | "rangeEnd" | "contextLine"> {
  return {
    r: range.id,
    rangeStart: isCustomLogRange(range) ? range.startTime : undefined,
    rangeEnd: isCustomLogRange(range) ? range.endTime : undefined,
    contextLine: undefined,
  };
}

/**
 * A project log view's View in context link for a line (w3/m528): the same
 * view, 10 minutes either side of the line, with the line marked. The view's
 * own filter document stays: it is the view, as Render's link keeps the page
 * it is on; only the search and filters typed over it are dropped.
 */
export function projectLogContextHref(
  projectId: string,
  viewType: SavedSearchType,
  savedSearchId: string | undefined,
  line: LogLine,
): string | undefined {
  const context = logContextSearch(line);
  if (!context) return undefined;
  const kind =
    viewType === SavedSearchType.RequestLogs ? "request" : "application";
  return searchHref(`/project/${projectId}/observability/logs/${kind}`, {
    savedSearchId,
    r: context.range,
    rangeStart: context.rangeStart,
    rangeEnd: context.rangeEnd,
    contextLine: context.contextLine,
  });
}

/**
 * A saved search's own default range when the URL names none: its
 * `timeRange.relative` (Render's shape, w3/083), or the `TIME_RANGE` row a
 * view bex stored before then holds. null when the view sets none, or one
 * bex has no preset for.
 */
export function viewTimeRange(
  filterDocument: Record<string, unknown> | undefined,
): RangePreset | null {
  return parseRangePreset(documentTimeRange(filterDocument));
}

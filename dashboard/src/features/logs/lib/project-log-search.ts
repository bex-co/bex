import {
  isCustomRange,
  parseCustomRange,
  parseRangePreset,
  type RangePreset,
  type RangePresetID,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { SavedSearchType } from "@/graphql/definitions";
import type { LogLine } from "../types";
import {
  logContextSearch,
  parseRangeAlias,
  searchHref,
  str,
} from "./log-search";

/**
 * A project observability view's URL (w3/m505), in Render's keys:
 * `/project/{prj}/observability/logs/application?savedSearchId=svs-…&r=1h`.
 * `savedSearchId` names the view and `r` the range; a custom range keeps the
 * Logs tab's `rangeStart`/`rangeEnd` beside `r=custom`. Every key comes back
 * (undefined when absent or rejected) because the router merges the validated
 * search over the raw one, and a key it omits lingers (w7/m42).
 */
export interface ProjectLogSearch {
  savedSearchId?: string;
  r?: RangePresetID | "custom";
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
  out.r = parseRangeAlias(search.r)?.id;
  return out;
}

/** The range the URL names, else the view's own default. */
export function projectLogRange(
  search: ProjectLogSearch,
  fallback: RangeSelection,
): RangeSelection {
  if (search.r === "custom" && search.rangeStart && search.rangeEnd) {
    const custom = parseCustomRange(search.rangeStart, search.rangeEnd);
    if (custom) return custom;
  }
  return parseRangePreset(search.r) ?? fallback;
}

/** A range as the URL's keys, every one written so a stale bound clears,
 *  and a View in context mark with it (w3/m528). */
export function projectLogRangeSearch(
  range: RangeSelection,
): Pick<ProjectLogSearch, "r" | "rangeStart" | "rangeEnd" | "contextLine"> {
  return {
    r: range.id,
    rangeStart: isCustomRange(range) ? range.startTime : undefined,
    rangeEnd: isCustomRange(range) ? range.endTime : undefined,
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
 * A saved search's `TIME_RANGE` filter (`IS` one preset): the view's own
 * default range when the URL names none. null when the view sets none, or
 * one bex has no preset for.
 */
export function viewTimeRange(
  filterDocument: Record<string, unknown> | undefined,
): RangePreset | null {
  const filters = filterDocument?.filters;
  if (!Array.isArray(filters)) return null;
  for (const filter of filters) {
    if (
      filter &&
      typeof filter === "object" &&
      (filter as { field?: unknown }).field === "TIME_RANGE" &&
      (filter as { operator?: unknown }).operator === "IS"
    ) {
      const values = (filter as { values?: unknown }).values;
      return Array.isArray(values) ? parseRangePreset(values[0]) : null;
    }
  }
  return null;
}

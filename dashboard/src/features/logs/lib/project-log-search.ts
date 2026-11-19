import {
  isCustomRange,
  parseCustomRange,
  parseRangePreset,
  type RangePreset,
  type RangePresetID,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { parseRangeAlias, str } from "./log-search";

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
}

export function parseProjectLogSearch(
  search: Record<string, unknown>,
): ProjectLogSearch {
  const out: ProjectLogSearch = {
    savedSearchId: str(search.savedSearchId),
    r: undefined,
    rangeStart: undefined,
    rangeEnd: undefined,
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

/** A range as the URL's keys, every one written so a stale bound clears. */
export function projectLogRangeSearch(
  range: RangeSelection,
): Pick<ProjectLogSearch, "r" | "rangeStart" | "rangeEnd"> {
  return {
    r: range.id,
    rangeStart: isCustomRange(range) ? range.startTime : undefined,
    rangeEnd: isCustomRange(range) ? range.endTime : undefined,
  };
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

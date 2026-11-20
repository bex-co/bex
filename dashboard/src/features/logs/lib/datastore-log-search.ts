import {
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
  type RangeSearch,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import type { LogLine } from "../types";
import { DEFAULT_DATASTORE_LOG_RANGE } from "./datastore-log-range";
import { logContextSearch, searchHref, str } from "./log-search";

// The datastore Logs tab's URL state (Postgres + Key Value, w4/228): the shared
// range keys (w6/065) plus the viewer's own filters — text, instance and, since
// both datastores' lines carry one (w5/m139), level — so
// a filtered datastore view survives a reload and a copied link like the
// service Logs tab's does (w7/m42). Deliberately not the service vocabulary —
// the datastore logs contract refuses service-only filters.
export interface DatastoreLogFilters {
  text: string;
  instance: string;
  level: string;
}

export interface DatastoreLogSearch extends RangeSearch {
  text?: string;
  instance?: string;
  level?: string;
  /** The line a View in context link marks (w3/m528): its key. */
  contextLine?: string;
}

/**
 * Validates the datastore Logs keys off a raw URL search. Every owned key comes
 * back, undefined when absent or rejected, and a numeric-looking text that the
 * router JSON-parsed into a number (`?text=404`) is accepted back as a string.
 */
export function parseDatastoreLogSearch(
  search: Record<string, unknown>,
): DatastoreLogSearch {
  return {
    ...parseRangeSearch(search),
    text: str(search.text),
    instance: str(search.instance),
    level: str(search.level),
    contextLine: str(search.contextLine),
  };
}

/**
 * A Postgres or Key Value Logs tab's View in context link for a line
 * (w3/m528): the tab with no search or pick, 10 minutes either side of the
 * line, with the line marked. Its Live switch is off by default already.
 */
export function datastoreLogContextHref(
  kind: "databases" | "keyvalue",
  resource: string,
  line: LogLine,
): string | undefined {
  const context = logContextSearch(line);
  return context
    ? searchHref(`/${kind}/${resource}`, { tab: "logs", ...context })
    : undefined;
}

/**
 * The viewer's URL-backed props for a hosting route. `write` merges a patch
 * over the current search; every key in a patch is explicit (undefined clears)
 * because the router retains params an update merely omits (w7/m42).
 */
export function datastoreLogSearchProps(
  search: DatastoreLogSearch,
  write: (patch: DatastoreLogSearch) => void,
): {
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
  filters: DatastoreLogFilters;
  onFiltersChange: (patch: Partial<DatastoreLogFilters>) => void;
  contextLine: string | undefined;
} {
  return {
    range: rangeFromSearch(search, DEFAULT_DATASTORE_LOG_RANGE),
    // Any change leaves a View in context link's mark behind (w3/m528).
    onRangeChange: (range) =>
      write({ ...rangeToSearch(range), contextLine: undefined }),
    filters: {
      text: search.text ?? "",
      instance: search.instance ?? "",
      level: search.level ?? "",
    },
    // Only the keys a patch names are written; an emptied one is cleared.
    onFiltersChange: (patch) =>
      write({
        ...Object.fromEntries(
          Object.entries(patch).map(([key, value]) => [
            key,
            value || undefined,
          ]),
        ),
        contextLine: undefined,
      }),
    contextLine: search.contextLine,
  };
}

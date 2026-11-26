import type { LogLine } from "../types";
import { DEFAULT_DATASTORE_LOG_RANGE } from "./datastore-log-range";
import {
  logRangeFromSearch,
  logRangeToSearch,
  parseLogRangeSearch,
  type LogRange,
  type LogRangeSearch,
} from "./log-range";
import { logContextSearch, searchHref, str } from "./log-search";

// The datastore Logs tab's URL state (Postgres + Key Value, w4/228): the shared
// range keys (w6/065), Live tail among them as `range=live` (w3/m537), plus the viewer's own filters — text, instance and, since
// both datastores' lines carry one (w5/m139), level — so
// a filtered datastore view survives a reload and a copied link like the
// service Logs tab's does (w7/m42). Deliberately not the service vocabulary —
// the datastore logs contract refuses service-only filters.
export interface DatastoreLogFilters {
  text: string;
  instance: string;
  level: string;
}

export interface DatastoreLogSearch extends LogRangeSearch {
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
    ...parseLogRangeSearch(search),
    text: str(search.text),
    instance: str(search.instance),
    level: str(search.level),
    contextLine: str(search.contextLine),
  };
}

/**
 * A Postgres or Key Value Logs page's View in context link for a line
 * (w3/m528): the page with no search or pick, 10 minutes either side of the
 * line, with the line marked. A custom range never tails. Each datastore's
 * Logs is its own page: `/databases/<id>/logs` (w3/m641) and
 * `/keyvalue/<id>/logs` (w3/m649).
 */
export function datastoreLogContextHref(
  kind: "databases" | "keyvalue",
  resource: string,
  line: LogLine,
): string | undefined {
  const context = logContextSearch(line);
  if (!context) return undefined;
  return searchHref(`/${kind}/${resource}/logs`, context);
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
  range: LogRange;
  onRangeChange: (range: LogRange) => void;
  filters: DatastoreLogFilters;
  onFiltersChange: (patch: Partial<DatastoreLogFilters>) => void;
  contextLine: string | undefined;
} {
  return {
    range: logRangeFromSearch(search, DEFAULT_DATASTORE_LOG_RANGE),
    // Any change leaves a View in context link's mark behind (w3/m528).
    onRangeChange: (range) =>
      write({ ...logRangeToSearch(range), contextLine: undefined }),
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

import {
  parseRangeSearch,
  rangeFromSearch,
  rangeToSearch,
  type RangeSearch,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import { DEFAULT_DATASTORE_LOG_RANGE } from "./datastore-log-range";
import { str } from "./log-search";

// The datastore Logs tab's URL state (Postgres + Key Value, w4/228): the shared
// range keys (w6/065) plus the viewer's own two filters, text and instance, so
// a filtered datastore view survives a reload and a copied link like the
// service Logs tab's does (w7/m42). Deliberately not the service vocabulary —
// the datastore logs contract refuses service-only filters.
export interface DatastoreLogFilters {
  text: string;
  instance: string;
}

export interface DatastoreLogSearch extends RangeSearch {
  text?: string;
  instance?: string;
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
  };
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
} {
  return {
    range: rangeFromSearch(search, DEFAULT_DATASTORE_LOG_RANGE),
    onRangeChange: (range) => write(rangeToSearch(range)),
    filters: { text: search.text ?? "", instance: search.instance ?? "" },
    // Only the keys a patch names are written; an emptied one is cleared.
    onFiltersChange: (patch) =>
      write(
        Object.fromEntries(
          Object.entries(patch).map(([key, value]) => [
            key,
            value || undefined,
          ]),
        ),
      ),
  };
}

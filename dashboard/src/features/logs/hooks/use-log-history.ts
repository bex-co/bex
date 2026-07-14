import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines } from "../lib/map";
import { LOG_TYPE_ALL, type LogFilters, type LogLine } from "../types";

// bex-api's GraphQL logs query defaults to 20 lines and caps at 100 (Render's
// paging range, internal/logs/service.go). The viewer asks for the max so the
// historical panel is as full as the contract allows before the live tail takes
// over.
const HISTORY_LIMIT = 100;

// The message bex-api returns when a request-log / structured-filter query hits
// a deployment with no durable store wired (core.ErrLogStoreUnavailable → 503).
// The viewer renders this as an explanatory state, not a generic error toast.
const STORE_UNAVAILABLE_MARKER = "durable log store";

export interface UseLogHistoryResult {
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
  /**
   * True when the query failed because it asked for request logs or a
   * structured filter the durable store owns, and the store isn't wired
   * (local dev). A distinct, non-error state — not "logs are broken".
   */
  storeUnavailable: boolean;
}

// A structured filter is single-valued in the UI; bex-api takes lists, so send a
// single-element list (or undefined for "no filter").
function list(value: string): string[] | undefined {
  return value ? [value] : undefined;
}

/**
 * Reads one App's historical logs from bex-api's `logs(resource, type, text,
 * level, instance, statusCode, method, path, limit)` query, in Render's
 * `LogEntry` shape (docs/ADR010-observability.md). Presentation only — the same shared
 * Core read the REST/MCP adapters use.
 *
 * `type=all` and an empty `text` are sent as absent args (the whole, unfiltered
 * page); the structured filters go through as single-element lists. Without the
 * durable store, request logs and structured filters resolve to `storeUnavailable`
 * rather than an error, per bex-api's honesty contract.
 */
export function useLogHistory(
  resource: string,
  filters: LogFilters,
): UseLogHistoryResult {
  const { data, loading, error } = useQuery(LogsDocument, {
    variables: {
      resource,
      type: filters.type === LOG_TYPE_ALL ? undefined : filters.type,
      text: filters.text || undefined,
      level: list(filters.level),
      instance: list(filters.instance),
      statusCode: list(filters.statusCode),
      method: list(filters.method),
      path: list(filters.path),
      limit: HISTORY_LIMIT,
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const lines = useMemo(() => toLogLines(data?.logs), [data]);
  const storeUnavailable =
    !!error && error.message.includes(STORE_UNAVAILABLE_MARKER);

  return { lines, loading, error, storeUnavailable };
}

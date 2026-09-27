import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";
import { dedupeLogLines, toLogLines } from "../lib/map";
import { useOlderLogPages } from "./use-older-log-pages";
import {
  LOG_PAGE_SIZE,
  LOG_TYPE_ALL,
  type LogFilters,
  type LogLine,
} from "../types";

// bex-api's GraphQL logs query defaults to 20 lines and caps at 100 (Render's
// paging range, internal/logs/service.go). The viewer asks for the max so the
// historical panel is as full as the contract allows before the live tail takes
// over.
const HISTORY_LIMIT = LOG_PAGE_SIZE;

// The message bex-api returns when a request-log / structured-filter query hits
// a deployment with no durable store wired (core.ErrLogStoreUnavailable → 503).
// The viewer renders this as an explanatory state, not a generic error toast.
const STORE_UNAVAILABLE_MARKER = "durable log store";

// bex-api's code for a read that ran out of its time budget (w4/m140).
const QUERY_TIMEOUT = "QUERY_TIMEOUT";

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
  /**
   * True when the search ran out of the server's time budget before covering
   * any of the range (`QUERY_TIMEOUT`, w4/m140). Retrying the same search times
   * out again, so the viewer asks for a narrower one instead.
   */
  timedOut: boolean;
  /** True when the server says more history exists older than the loaded pages. */
  hasMore: boolean;
  loadingOlder: boolean;
  /** Fetch the next older page and prepend it. No-op when !hasMore. */
  loadOlder: () => void;
}

// A structured filter is single-valued in the UI; bex-api takes lists, so send a
// single-element list (or undefined for "no filter").
function list(value: string): string[] | undefined {
  return value ? [value] : undefined;
}

/**
 * Reads one App's historical logs from bex-api's `logs(resource, type, text,
 * level, instance, statusCode, method, path, limit)` query, in Render's
 * paging envelope (docs/ADR010-observability.md). Presentation only — the same
 * shared Core read the REST/MCP adapters use.
 *
 * `type=all` and an empty `text` are sent as absent args (the whole, unfiltered
 * page); the structured filters go through as single-element lists. Without the
 * durable store, request logs and structured filters resolve to `storeUnavailable`
 * rather than an error, per bex-api's honesty contract.
 *
 * Older pages (scroll-to-top) are fetched with the envelope's cursors and kept
 * in local state — not in the URL (w4/m107); `useOlderLogPages` owns them.
 */
export function useLogHistory(
  resource: string,
  filters: LogFilters,
  window?: { startTime: string; endTime: string },
): UseLogHistoryResult {
  const variables = useMemo(
    () => ({
      resource,
      type: filters.type === LOG_TYPE_ALL ? undefined : filters.type,
      text: filters.text || undefined,
      level: list(filters.level),
      instance: list(filters.instance),
      statusCode: list(filters.statusCode),
      method: list(filters.method),
      path: list(filters.path),
      startTime: window?.startTime,
      endTime: window?.endTime,
      limit: HISTORY_LIMIT,
    }),
    [
      resource,
      filters.type,
      filters.text,
      filters.level,
      filters.instance,
      filters.statusCode,
      filters.method,
      filters.path,
      window?.startTime,
      window?.endTime,
    ],
  );

  const { data, loading, error } = useQuery(LogsDocument, {
    variables,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const pages = useOlderLogPages(variables, data?.logs);

  const firstPage = useMemo(
    () => toLogLines(data?.logs?.logs),
    [data?.logs?.logs],
  );
  const lines = useMemo(
    () => dedupeLogLines([...pages.older, ...firstPage]),
    [pages.older, firstPage],
  );

  const storeUnavailable =
    !!error && error.message.includes(STORE_UNAVAILABLE_MARKER);
  const timedOut = hasGraphQLErrorCode(error, QUERY_TIMEOUT);

  return {
    lines,
    loading: loading && lines.length === 0,
    error,
    storeUnavailable,
    timedOut,
    hasMore: pages.hasMore,
    loadingOlder: pages.loadingOlder,
    loadOlder: pages.loadOlder,
  };
}

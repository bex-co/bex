import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines } from "../lib/map";
import { LOG_TYPE_ALL, type LogLine, type LogTypeFilter } from "../types";

// bex-api's GraphQL logs query defaults to 20 lines and caps at 100 (Render's
// paging range, internal/logs/service.go). The viewer asks for the max so the
// historical panel is as full as the contract allows before the live tail takes
// over.
const HISTORY_LIMIT = 100;

export interface UseLogHistoryOptions {
  resource: string;
  type: LogTypeFilter;
  /** Case-insensitive substring, applied server-side via the `text` arg. */
  text: string;
}

export interface UseLogHistoryResult {
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads one App's historical logs from bex-api's `logs(resource, type, text,
 * limit)` query, in Render's `LogEntry` shape (docs/observability.md).
 * Presentation only — the same shared Core read the REST/MCP adapters use.
 *
 * `type=all` and an empty `text` are sent as absent args (the whole, unfiltered
 * page); `request` resolves to an empty result per bex-api's contract, not an
 * error.
 */
export function useLogHistory({
  resource,
  type,
  text,
}: UseLogHistoryOptions): UseLogHistoryResult {
  const { data, loading, error } = useQuery(LogsDocument, {
    variables: {
      resource,
      type: type === LOG_TYPE_ALL ? undefined : type,
      text: text || undefined,
      limit: HISTORY_LIMIT,
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const lines = useMemo(() => toLogLines(data?.logs), [data]);

  return { lines, loading, error };
}

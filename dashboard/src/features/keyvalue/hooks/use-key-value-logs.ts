import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines } from "@/features/logs/lib/map";
import type { LogLine } from "@/features/logs/types";

// Window-based since w5/030 — the datastore Logs viewers share the m56 range
// ladder (via RangeSelect), so their range control is a resolved
// [startTime, endTime] window, not a private 1h/6h/24h enum.
export interface KeyValueLogFilters {
  startTime: string;
  endTime: string;
  text: string;
  instance: string;
}

export interface UseKeyValueLogsResult {
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
  unavailable: boolean;
  unauthorized: boolean;
}

/**
 * Reads one managed Key Value (Valkey) resource through the same generic
 * GraphQL logs query used by services. Only the datastore contract's window,
 * text, and instance filters are sent.
 */
export function useKeyValueLogs(
  resource: string,
  filters: KeyValueLogFilters,
): UseKeyValueLogsResult {
  const { data, loading, error } = useQuery(LogsDocument, {
    variables: {
      resource,
      text: filters.text || undefined,
      instance: filters.instance ? [filters.instance] : undefined,
      startTime: filters.startTime,
      endTime: filters.endTime,
      limit: 100,
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const lines = useMemo(() => toLogLines(data?.logs), [data]);
  const message = error?.message.toLowerCase() ?? "";
  return {
    lines,
    loading,
    error,
    unavailable: message.includes("logs source not configured"),
    unauthorized: message.includes("forbidden"),
  };
}

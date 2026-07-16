import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines } from "@/features/logs/lib/map";
import type { LogLine } from "@/features/logs/types";

export type PostgresLogRange = "1h" | "6h" | "24h";

export interface PostgresLogFilters {
  range: PostgresLogRange;
  text: string;
  instance: string;
}

export interface UsePostgresLogsResult {
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
  unavailable: boolean;
  unauthorized: boolean;
}

const RANGE_MS: Record<PostgresLogRange, number> = {
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
};

/**
 * Reads one managed Postgres resource through the same generic GraphQL logs
 * query used by services. Only the datastore contract's range, text, and
 * instance filters are sent; no service/request filter is silently reused.
 */
export function usePostgresLogs(
  resource: string,
  filters: PostgresLogFilters,
): UsePostgresLogsResult {
  const window = useMemo(() => {
    const end = new Date();
    return {
      startTime: new Date(
        end.getTime() - RANGE_MS[filters.range],
      ).toISOString(),
      endTime: end.toISOString(),
    };
  }, [filters.range]);

  const { data, loading, error } = useQuery(LogsDocument, {
    variables: {
      resource,
      text: filters.text || undefined,
      instance: filters.instance ? [filters.instance] : undefined,
      startTime: window.startTime,
      endTime: window.endTime,
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

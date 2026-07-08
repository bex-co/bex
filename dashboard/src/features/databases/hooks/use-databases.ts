import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DatabasesDocument } from "@/graphql/definitions";
import { toDatabaseViews } from "@/features/databases/lib/status";
import type { DatabaseView } from "@/features/databases/types";

export interface UseDatabasesResult {
  databases: DatabaseView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (fire-and-forget; callers refresh the list after a write). */
  refetch: () => Promise<unknown>;
  /** Begin Apollo polling at the given cadence (while a row is still creating). */
  startPolling: (ms: number) => void;
  stopPolling: () => void;
}

/**
 * Reads bex-api's `databases` query and maps each Render-shaped `Database` onto
 * a normalized DatabaseView. Presentation only — the list is the operator's real
 * Database CRs (docs/bex-api.md §Managed Postgres); mirrors `useServices`.
 */
export function useDatabases(): UseDatabasesResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    DatabasesDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );

  const databases = useMemo(() => toDatabaseViews(data?.databases), [data]);

  return { databases, loading, error, refetch, startPolling, stopPolling };
}

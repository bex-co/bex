import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DatabaseDocument } from "@/graphql/definitions";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import {
  toDatabaseDetailView,
  isConverging,
} from "@/features/databases/lib/status";
import type { DatabaseDetailView } from "@/features/databases/types";

export interface UseDatabaseResult {
  database: DatabaseDetailView | null;
  loading: boolean;
  error: Error | undefined;
  /** Re-read the database now (used after a lifecycle verb converges the state). */
  refetch: () => void;
}

/**
 * Reads bex-api's `database(id)` query for the detail page. Polls only while the
 * row is still provisioning (or not yet loaded) so the header converges to
 * Available on its own, then stops — an idle, settled DB won't change, so
 * refetching it forever is waste. Mirrors the list page's gated poll. Connection
 * info is NOT fetched here — it's revealed on demand from the detail page
 * (docs/ADR006-bex-api.md §Managed Postgres: the password is surfaced only on request).
 */
export function useDatabase(id: string): UseDatabaseResult {
  const { data, loading, error, startPolling, stopPolling, refetch } = useQuery(
    DatabaseDocument,
    { variables: { id }, fetchPolicy: "cache-first", errorPolicy: "all" },
  );

  const database = useMemo(
    () => (data?.database ? toDatabaseDetailView(data.database) : null),
    [data],
  );

  // Poll fast until we know the DB is settled: while it hasn't loaded yet, or
  // while it's still creating. Once available/unavailable, fall back to the
  // baseline cadence so out-of-band changes still show up.
  const converging = database ? isConverging(database) : true;
  useEffect(() => {
    startPolling(converging ? 3000 : RESOURCE_POLL_INTERVAL_MS);
    return () => stopPolling();
  }, [converging, startPolling, stopPolling]);

  return {
    database,
    loading,
    error,
    refetch: () => {
      // A successful lifecycle mutation updates desired state before the
      // operator necessarily exposes its first converging status. Begin polling
      // immediately so an eager refetch that still says "available" cannot hide
      // the subsequent Upgrading/Provisioning transition.
      startPolling(3000);
      void refetch();
    },
  };
}

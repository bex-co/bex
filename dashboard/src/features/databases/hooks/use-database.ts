import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DatabaseDocument } from "@/graphql/definitions";
import { eagerRefetch, useConvergingPoll } from "@/common/lib/polling";
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
export function useDatabase(
  id: string,
  {
    poll = true,
  }: {
    /**
     * Pass `false` on a secondary consumer (the topbar breadcrumb) mounted
     * beside the page that owns polling: it reads the same cached query, and a
     * second timer would drift into its own round trips.
     */
    poll?: boolean;
  } = {},
): UseDatabaseResult {
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
  useConvergingPoll(
    startPolling,
    stopPolling,
    database ? isConverging(database) : true,
    poll,
  );

  return {
    database,
    loading,
    error,
    refetch: () => eagerRefetch(startPolling, refetch),
  };
}

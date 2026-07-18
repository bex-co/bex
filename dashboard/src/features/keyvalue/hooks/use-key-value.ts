import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { KeyValueDocument } from "@/graphql/definitions";
import { toKeyValueView, isConverging } from "@/features/keyvalue/lib/status";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface UseKeyValueResult {
  keyValue: KeyValueView | null;
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (used after suspend/resume to pick up the fresh state). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `keyValue(id)` query for the detail page. Polls only while the
 * row is still provisioning (or not yet loaded) so the header converges to
 * Available on its own, then stops — mirrors the list page's gated poll and
 * databases' `useDatabase`. Connection info is NOT fetched here — it's revealed
 * on demand from the detail page (docs/ADR021-keyvalue-management.md).
 */
export function useKeyValue(id: string): UseKeyValueResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    KeyValueDocument,
    { variables: { id }, fetchPolicy: "cache-first", errorPolicy: "all" },
  );

  const keyValue = useMemo(
    () => (data?.keyValue ? toKeyValueView(data.keyValue) : null),
    [data],
  );

  // Poll until we know the store is settled: while it hasn't loaded yet, or
  // while it's still creating. Once available/unavailable, stop.
  const converging = keyValue ? isConverging(keyValue) : true;
  useEffect(() => {
    if (converging) startPolling(3000);
    else stopPolling();
    return () => stopPolling();
  }, [converging, startPolling, stopPolling]);

  return { keyValue, loading, error, refetch };
}

import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { KeyValueDocument } from "@/graphql/definitions";
import { eagerRefetch, useConvergingPoll } from "@/common/lib/polling";
import { toKeyValueView, isConverging } from "@/features/keyvalue/lib/status";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface UseKeyValueResult {
  keyValue: KeyValueView | null;
  loading: boolean;
  error: Error | undefined;
  /** Re-read the store now (used after a lifecycle verb converges the state). */
  refetch: () => void;
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

  // Poll fast until we know the store is settled: while it hasn't loaded yet,
  // or while it's still creating. Once available/unavailable, fall back to the
  // baseline cadence so out-of-band changes still show up.
  useConvergingPoll(
    startPolling,
    stopPolling,
    keyValue ? isConverging(keyValue) : true,
  );

  return {
    keyValue,
    loading,
    error,
    refetch: () => eagerRefetch(startPolling, refetch),
  };
}

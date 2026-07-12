import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { KeyValuesDocument } from "@/graphql/definitions";
import { toKeyValueViews } from "@/features/keyvalue/lib/status";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface UseKeyValuesResult {
  keyValues: KeyValueView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (fire-and-forget; callers refresh the list after a write). */
  refetch: () => Promise<unknown>;
  /** Begin Apollo polling at the given cadence (while a row is still creating). */
  startPolling: (ms: number) => void;
  stopPolling: () => void;
}

/**
 * Reads bex-api's `keyValues` query and maps each Render-shaped `KeyValue` onto
 * a normalized KeyValueView. Presentation only — the list is the operator's real
 * KeyValue CRs (docs/ADR021-keyvalue-management.md); mirrors `useDatabases`.
 */
export function useKeyValues(): UseKeyValuesResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    KeyValuesDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );

  const keyValues = useMemo(() => toKeyValueViews(data?.keyValues), [data]);

  return { keyValues, loading, error, refetch, startPolling, stopPolling };
}

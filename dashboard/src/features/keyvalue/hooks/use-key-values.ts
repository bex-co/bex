import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { KeyValuesDocument } from "@/graphql/definitions";
import { toKeyValueViews } from "@/features/keyvalue/lib/status";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

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
 * Scoped to the switcher's selected workspace (w6/m14) — the create side already
 * writes into it, so the list has to read from it too, or a store created in
 * workspace B lands in a list showing every workspace's stores. Skipped until
 * the selection resolves, exactly like useServices/useDatabases.
 */
export function useKeyValues(): UseKeyValuesResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    KeyValuesDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      skip: !resolved,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
    },
  );

  const keyValues = useMemo(() => toKeyValueViews(data?.keyValues), [data]);

  return {
    keyValues,
    loading: !resolved || loading,
    error,
    refetch,
    startPolling,
    stopPolling,
  };
}

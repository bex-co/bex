import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { KeyValueInstanceTypesDocument } from "@/graphql/definitions";
import type { KeyValueInstanceTypeView } from "@/features/keyvalue/types";

export interface UseKeyValueInstanceTypesResult {
  /** The Valkey catalog, in ladder order (empty while loading or on error). */
  instanceTypes: KeyValueInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads bex-api's `keyValueInstanceTypes` (w5/m12, backed by lego/types/tiers'
 * Valkey family) — the create form's plan-picker source. A bex extension
 * (Render's dashboard hardcodes its own Key Value plan list), so there is
 * nothing to poll against; the default cache-first policy is fine. Never a
 * hardcoded ladder here — the plans come from the shared catalog.
 */
export function useKeyValueInstanceTypes(): UseKeyValueInstanceTypesResult {
  const { data, loading, error } = useQuery(KeyValueInstanceTypesDocument);

  const instanceTypes = useMemo<KeyValueInstanceTypeView[]>(
    () =>
      (data?.keyValueInstanceTypes ?? [])
        .filter((t): t is NonNullable<typeof t> => t?.id != null)
        .map((t) => ({
          id: t.id!,
          name: t.name ?? t.id!,
          cpu: t.cpu ?? "",
          memory: t.memory ?? "",
          storageGB: t.storageGB ?? 0,
        })),
    [data],
  );

  return { instanceTypes, loading, error };
}

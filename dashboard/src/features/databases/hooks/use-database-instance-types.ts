import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { DatabaseInstanceTypesDocument } from "@/graphql/definitions";
import type { DatabaseInstanceTypeView } from "@/features/databases/types";

export interface UseDatabaseInstanceTypesResult {
  /** The Postgres catalog, in ladder order (empty while loading or on error). */
  instanceTypes: DatabaseInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads bex-api's `databaseInstanceTypes` (w5/m8, backed by lego/types/tiers'
 * Postgres family) — the create dialog's plan-picker source. A bex extension
 * (Render's dashboard hardcodes its own list), so there is nothing to poll
 * against; the default cache-first policy is fine. Never a hardcoded ladder
 * here — the plans come from the shared catalog.
 */
export function useDatabaseInstanceTypes(): UseDatabaseInstanceTypesResult {
  const { data, loading, error } = useQuery(DatabaseInstanceTypesDocument);

  const instanceTypes = useMemo<DatabaseInstanceTypeView[]>(
    () =>
      (data?.databaseInstanceTypes ?? [])
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

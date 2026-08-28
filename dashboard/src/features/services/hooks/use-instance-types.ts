import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { InstanceTypesDocument } from "@/graphql/definitions";

/** One catalog tier, as the picker and Settings row consume it. */
export interface InstanceTypeView {
  /** Render's plan spelling — what updateServicePlan accepts. */
  id: string;
  name: string;
  cpu: string;
  memory: string;
  /** Always-on 730-hour estimate from bex-api's canonical price sheet. */
  monthlyUsd: string;
}

export interface UseInstanceTypesResult {
  /** The catalog, in ladder order (empty while loading or on error). */
  instanceTypes: InstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
  /** O(1) lookup by Render plan id — undefined for an unknown/empty plan. */
  byID: (id: string | null | undefined) => InstanceTypeView | undefined;
}

/**
 * Reads bex-api's `instanceTypes` (w5/m7, backed by lego/types/tiers) — the
 * plan picker's data source. A bex extension: Render's dashboard hardcodes its
 * own instance-type list, so there is nothing to poll against; this query
 * rarely changes, so the default Apollo cache-first policy is fine.
 */
export function useInstanceTypes(): UseInstanceTypesResult {
  const { data, loading, error } = useQuery(InstanceTypesDocument);

  const instanceTypes = useMemo<InstanceTypeView[]>(
    () =>
      (data?.instanceTypes ?? [])
        .filter((t): t is NonNullable<typeof t> => t?.id != null)
        .map((t) => ({
          id: t.id!,
          name: t.name ?? t.id!,
          cpu: t.cpu ?? "",
          memory: t.memory ?? "",
          monthlyUsd: t.monthlyUsd ?? "",
        })),
    [data],
  );

  const byID = (id: string | null | undefined) =>
    id ? instanceTypes.find((t) => t.id === id) : undefined;

  return { instanceTypes, loading, error, byID };
}

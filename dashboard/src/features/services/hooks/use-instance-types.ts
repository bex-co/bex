import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { useCatalogRetry } from "@/common/hooks/use-catalog-retry";
import { useTranslations } from "@/common/hooks/use-translations";
import { withPausedReasons } from "@/common/lib/paused-tier";
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
  /**
   * The rate per minute of run time, a cron job's price: it is billed per
   * second only while a run executes (w4/546). "" from an older bex-api.
   * Optional here; the hook always sets it.
   */
  usdPerMinute?: string;
  /**
   * False for a temporarily paused tier (w2/m250): still listed so a service
   * already on it renders, but never offered as a new selection — bex-api
   * refuses it with INSTANCE_TYPE_UNAVAILABLE.
   */
  available: boolean;
  /**
   * Why the tier can't be chosen, in the dashboard's language, "" when
   * available. bex-api's own reason is English only, so it is used only when
   * no tier is left to suggest (w4/552).
   */
  unavailableReason: string;
}

export interface UseInstanceTypesResult {
  /** The catalog, in ladder order (empty while loading or on error). */
  instanceTypes: InstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
  /** O(1) lookup by Render plan id — undefined for an unknown/empty plan. */
  byID: (id: string | null | undefined) => InstanceTypeView | undefined;
  /** Re-reads the catalog in place; busy in `retrying` (w4/m218). */
  retry: () => Promise<void>;
  retrying: boolean;
}

/**
 * Reads bex-api's `instanceTypes` (w5/m7, backed by lego/types/tiers) — the
 * plan picker's data source. A bex extension: Render's dashboard hardcodes its
 * own instance-type list, so there is nothing to poll against; this query
 * rarely changes, so the default Apollo cache-first policy is fine.
 */
export function useInstanceTypes(): UseInstanceTypesResult {
  const { data, loading, error, refetch } = useQuery(InstanceTypesDocument);
  const { retry, retrying } = useCatalogRetry(refetch);
  const { t } = useTranslations();

  const instanceTypes = useMemo<InstanceTypeView[]>(() => {
    const tiers = (data?.instanceTypes ?? [])
      .filter((it): it is NonNullable<typeof it> => it?.id != null)
      .map((it) => ({
        id: it.id!,
        name: it.name ?? it.id!,
        cpu: it.cpu ?? "",
        memory: it.memory ?? "",
        monthlyUsd: it.monthlyUsd ?? "",
        usdPerMinute: it.usdPerMinute ?? "",
        // An older bex-api without the field lists every tier as selectable.
        available: it.available ?? true,
        unavailableReason: it.unavailableReason ?? "",
      }));
    return withPausedReasons(tiers, (name, largest) =>
      t("services.instanceTypePausedReason", { name, largest }),
    );
  }, [data, t]);

  const byID = (id: string | null | undefined) =>
    id ? instanceTypes.find((t) => t.id === id) : undefined;

  return { instanceTypes, loading, error, byID, retry, retrying };
}

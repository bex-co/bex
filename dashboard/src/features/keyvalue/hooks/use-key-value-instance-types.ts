import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { useCatalogRetry } from "@/common/hooks/use-catalog-retry";
import { useTranslations } from "@/common/hooks/use-translations";
import { withPausedReasons } from "@/common/lib/paused-tier";
import { KeyValueInstanceTypesDocument } from "@/graphql/definitions";
import type { KeyValueInstanceTypeView } from "@/features/keyvalue/types";

export interface UseKeyValueInstanceTypesResult {
  /** The Valkey catalog, in ladder order (empty while loading or on error). */
  instanceTypes: KeyValueInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-reads the catalog in place; busy in `retrying` (w4/m218). */
  retry: () => Promise<void>;
  retrying: boolean;
}

/**
 * Reads bex-api's `keyValueInstanceTypes` (w5/m12, backed by lego/types/tiers'
 * Valkey family) — the create form's plan-picker source. A bex extension
 * (Render's dashboard hardcodes its own Key Value plan list), so there is
 * nothing to poll against; the default cache-first policy is fine. Never a
 * hardcoded ladder here — the plans come from the shared catalog.
 */
export function useKeyValueInstanceTypes(): UseKeyValueInstanceTypesResult {
  const { data, loading, error, refetch } = useQuery(
    KeyValueInstanceTypesDocument,
  );
  const { retry, retrying } = useCatalogRetry(refetch);
  const { t } = useTranslations();

  const instanceTypes = useMemo<KeyValueInstanceTypeView[]>(() => {
    const tiers = (data?.keyValueInstanceTypes ?? [])
      .filter((it): it is NonNullable<typeof it> => it?.id != null)
      .map((it) => ({
        id: it.id!,
        name: it.name ?? it.id!,
        cpu: it.cpu ?? "",
        memory: it.memory ?? "",
        storageGB: it.storageGB ?? 0,
        monthlyUsd: it.monthlyUsd ?? "",
        available: it.available,
        unavailableReason: it.unavailableReason,
      }));
    return withPausedReasons(tiers, (name, largest) =>
      t("keyvalue.planPausedReason", { name, largest }),
    );
  }, [data, t]);

  return { instanceTypes, loading, error, retry, retrying };
}

import { useCallback } from "react";
import { useQuery } from "@apollo/client/react";
import { ServicesDocument } from "@/graphql/definitions";
import { toServiceView } from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";

export interface UseServicesResult {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query, resolving to the fresh list (used to poll to convergence). */
  refetch: () => Promise<ServiceView[]>;
}

/**
 * Reads bex-api's `services` query and maps each Render-shaped `Service` onto a
 * normalized ServiceView. Presentation only — the list is the operator's real
 * Apps (docs/bex-api.md); this mirrors the metrics hook's shared-Core read.
 */
export function useServices(): UseServicesResult {
  const { data, loading, error, refetch } = useQuery(ServicesDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const services = (data?.services ?? [])
    .filter((s) => s != null)
    .map((s) => toServiceView(s!));

  const refetchViews = useCallback(async () => {
    const res = await refetch();
    return (res.data?.services ?? [])
      .filter((s) => s != null)
      .map((s) => toServiceView(s!));
  }, [refetch]);

  return { services, loading, error, refetch: refetchViews };
}

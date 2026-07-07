import { useQuery } from "@apollo/client/react";
import { MonthToDateBandwidthDocument } from "@/graphql/definitions";

export interface UseMonthToDateBandwidthResult {
  /** bex only meters HTTP egress (Traefik-scraped) — null while loading. */
  egressBandwidthMB: number | null;
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads bex-api's monthToDateBandwidth(resourceId) — Render's "Usage this
 * month" bandwidth footer (captured live from its dashboard), shown alongside
 * the Outbound Bandwidth chart.
 */
export function useMonthToDateBandwidth(
  resource: string,
): UseMonthToDateBandwidthResult {
  const { data, loading, error } = useQuery(MonthToDateBandwidthDocument, {
    variables: { resourceId: resource },
    pollInterval: 60_000,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  return {
    egressBandwidthMB: data?.monthToDateBandwidth?.egressBandwidthMB ?? null,
    loading,
    error,
  };
}

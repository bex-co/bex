import { useQuery } from "@apollo/client/react";
import { MonthToDateBandwidthDocument } from "@/graphql/definitions";

export interface UseMonthToDateBandwidthResult {
  /** Composed public outbound bytes across every applicable source. */
  egressBandwidthMB: number | null;
  /** Sources whose health product failed inside the month window (w1/m50) —
   * the MB figure still includes what they recorded. */
  degradedSources: string[];
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
    degradedSources: (data?.monthToDateBandwidth?.degradedSources ?? []).filter(
      (s): s is string => !!s,
    ),
    loading,
    error,
  };
}

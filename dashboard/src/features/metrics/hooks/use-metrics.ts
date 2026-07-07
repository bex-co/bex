import { useQuery } from "@apollo/client/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MetricsDocument } from "@/graphql/definitions";
import type { MetricId, ChartSeries } from "@/features/metrics/types";

// bex-api's Metrics verb reports this exact message (Core's ErrMetricsUnavailable,
// operator/internal/api/core.go) when a metric's backend isn't wired (e.g. no
// Prometheus for request metrics) — surfaced here, not as a generic error.
export const METRICS_UNAVAILABLE_MESSAGE = "metrics source not configured";

export interface UseMetricsOptions {
  startTime?: string;
  endTime?: string;
  resolutionSeconds?: number;
  quantile?: number;
  percentage?: boolean;
  /** Polling cadence; 0 disables polling. Defaults to 30s. */
  pollIntervalMs?: number;
}

export interface UseMetricsResult {
  series: ChartSeries[];
  loading: boolean;
  /** true when bex-api reported ErrMetricsUnavailable (no backend wired). */
  unavailable: boolean;
  /** Any other error (network, auth, ...). */
  error: Error | undefined;
}

/**
 * Reads one bex-api metric for one App via GraphQL, polling by default so the
 * page reflects a live cluster. Mirrors the REST/MCP adapters' shared Core
 * read (docs/observability.md) — this hook is presentation only.
 */
export function useMetrics(
  resource: string,
  metric: MetricId,
  opts: UseMetricsOptions = {},
): UseMetricsResult {
  const { pollIntervalMs = 30_000, ...queryOpts } = opts;

  const { data, loading, error } = useQuery(MetricsDocument, {
    variables: { resource, metric, ...queryOpts },
    pollInterval: pollIntervalMs,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = Boolean(
    error &&
    CombinedGraphQLErrors.is(error) &&
    error.errors.some((e) => e.message === METRICS_UNAVAILABLE_MESSAGE),
  );

  const series: ChartSeries[] = (data?.metrics ?? [])
    .filter((s) => s != null)
    .map((s) => ({
      unit: s.unit ?? "",
      labels: Object.fromEntries(
        (s.labels ?? [])
          .filter((l) => l?.field != null && l.value != null)
          .map((l) => [l!.field as string, l!.value as string]),
      ),
      points: (s.points ?? [])
        .filter((p) => p?.timestamp != null && p.value != null)
        .map((p) => ({
          timestamp: p!.timestamp as string,
          value: p!.value as number,
        })),
    }));

  return {
    series,
    loading,
    unavailable,
    error: unavailable ? undefined : error,
  };
}

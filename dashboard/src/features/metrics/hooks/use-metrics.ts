import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MetricsDocument } from "@/graphql/definitions";
import {
  RENDER_METRIC_NAMES,
  type MetricId,
  type ChartSeries,
} from "@/features/metrics/types";

// bex-api's Metrics verb reports this exact message (Core's ErrMetricsUnavailable,
// operator/internal/api/core.go) when a metric's backend isn't wired (e.g. no
// Prometheus for request metrics) — surfaced here, not as a generic error.
export const METRICS_UNAVAILABLE_MESSAGE = "metrics source not configured";

export interface UseMetricsOptions {
  startTime?: string;
  endTime?: string;
  resolutionSeconds?: number;
  quantile?: number;
  /**
   * aggregateAllMethod: MAX — collapses a per-instance series (cpu_limit/
   * memory_limit) into one series holding the max across instances. Render's
   * dashboard sends this for the Limit query it fetches alongside the raw
   * metric, then computes Percentage/Total client-side from the two
   * (captured live: no server-side percentage flag exists in the real
   * contract) — see application-metrics-card.tsx.
   */
  aggregateMax?: boolean;
  /**
   * Render's toolbar Status Code filter: a class ("2xx"/"5xx") or exact code
   * ("500"), sent as a STATUS_CODE filters entry alongside RESOURCE.
   */
  statusCode?: string;
  /**
   * Render's per-chart "Group by": breaks the series out per status code or
   * method, sent as aggregateBy (the captured vocabulary — bex-api maps it
   * onto Core's groupBy exactly like REST's `groupBy` param).
   */
  groupBy?: "status" | "method";
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
 * read (docs/ADR010-observability.md) — this hook is presentation only.
 *
 * The GraphQL query shape mirrors Render's dashboard MetricsQueryInput
 * (captured live): a filters array carrying the RESOURCE entry, rather than a
 * flat `resource` argument.
 */
export function useMetrics(
  resource: string,
  metric: MetricId,
  opts: UseMetricsOptions = {},
): UseMetricsResult {
  const {
    pollIntervalMs = 30_000,
    startTime,
    endTime,
    resolutionSeconds,
    quantile,
    aggregateMax,
    statusCode,
    groupBy,
  } = opts;

  const { data, loading, error } = useQuery(MetricsDocument, {
    variables: {
      query: {
        filters: [
          { field: "RESOURCE", values: [resource] },
          ...(statusCode
            ? [{ field: "STATUS_CODE", values: [statusCode] }]
            : []),
        ],
        name: RENDER_METRIC_NAMES[metric],
        start: startTime,
        end: endTime,
        resolution: resolutionSeconds,
        parameters: quantile != null ? [{ quantile }] : undefined,
        aggregateBy:
          groupBy === "status"
            ? ["STATUS_CODE"]
            : groupBy === "method"
              ? ["METHOD"]
              : undefined,
        aggregateAllMethod: aggregateMax ? "MAX" : undefined,
      },
    },
    pollInterval: pollIntervalMs,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = Boolean(
    error &&
    CombinedGraphQLErrors.is(error) &&
    error.errors.some((e) => e.message === METRICS_UNAVAILABLE_MESSAGE),
  );

  // Memoized on data identity: a stable series identity is what lets the
  // charts' geometry useMemos actually cache across poll-tick re-renders.
  const series: ChartSeries[] = useMemo(
    () =>
      (data?.metrics ?? [])
        .filter((s) => s != null)
        .map((s) => ({
          unit: s.unit ?? "",
          labels: Object.fromEntries(
            (s.labels ?? [])
              .filter((l) => l?.field != null && l.value != null)
              .map((l) => [l!.field as string, l!.value as string]),
          ),
          points: (s.values ?? [])
            .filter((p) => p?.time != null && p.value != null)
            .map((p) => ({
              timestamp: p!.time as string,
              value: p!.value as number,
            })),
        })),
    [data],
  );

  return {
    series,
    loading,
    unavailable,
    error: unavailable ? undefined : error,
  };
}

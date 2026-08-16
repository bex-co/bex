import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { MetricsDocument } from "@/graphql/definitions";
import {
  RENDER_METRIC_NAMES,
  type MetricId,
  type ChartSeries,
} from "@/features/metrics/types";
import {
  isMetricsUnavailable,
  isLogStoreUnavailable,
  toChartSeries,
  METRICS_UNAVAILABLE_MESSAGE,
} from "@/features/metrics/lib/graphql-series";

// Re-exported for existing importers (this hook's own message constant lived
// here before it was shared with useDatastoreMetrics).
export { METRICS_UNAVAILABLE_MESSAGE };

export interface UseMetricsOptions {
  startTime?: string;
  endTime?: string;
  resolutionSeconds?: number;
  quantile?: number;
  /**
   * Several http_latency percentiles to read together — the percentile "All"
   * overlay (w5/m56). When non-empty, takes precedence over `quantile`: bex-api
   * returns one series per quantile, each tagged with a `quantile` label so the
   * chart can name the overlaid p50/p90/p99 lines.
   */
  quantiles?: number[];
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
  /**
   * Render's Network-card Host / Path filters (w5/m58). Sent as HOST / PATH
   * filters entries alongside RESOURCE; they apply only to http_requests /
   * http_latency. Traefik's Prometheus counters carry no host/path axis, so
   * bex-api serves a host/path-filtered read from the request-log store (Loki)
   * instead — with no store wired the read reports storeUnavailable (503), never
   * a silently-unfiltered result.
   */
  host?: string;
  path?: string;
  /** Polling cadence; 0 disables polling. Defaults to 30s. */
  pollIntervalMs?: number;
}

export interface UseMetricsResult {
  series: ChartSeries[];
  loading: boolean;
  /** true when bex-api reported ErrMetricsUnavailable (no backend wired). */
  unavailable: boolean;
  /**
   * true when a host/path-filtered request read hit a deployment with no durable
   * log store (bex-api's ErrLogStoreUnavailable → 503, w5/m58). The Network card
   * renders this as an explicit "needs the log store" state, never a silently
   * unfiltered chart — the Logs-tab 503 pattern.
   */
  storeUnavailable: boolean;
  /** Any other error (network, auth, ...). */
  error: Error | undefined;
  /**
   * Egress sources whose health product failed inside the window — bex-api's
   * `degraded_sources` series label on BANDWIDTH (w1/m50, ADR023 §
   * Observability reads vs billing reads). Empty for healthy windows and for
   * every non-bandwidth metric. The series still carries data; it may be
   * undercounted around the gap.
   */
  degradedSources: string[];
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
    quantiles,
    aggregateMax,
    statusCode,
    groupBy,
    host,
    path,
  } = opts;

  const { data, loading, error } = useQuery(MetricsDocument, {
    variables: {
      query: {
        filters: [
          { field: "RESOURCE", values: [resource] },
          ...(statusCode
            ? [{ field: "STATUS_CODE", values: [statusCode] }]
            : []),
          ...(host ? [{ field: "HOST", values: [host] }] : []),
          ...(path ? [{ field: "PATH", values: [path] }] : []),
        ],
        name: RENDER_METRIC_NAMES[metric],
        start: startTime,
        end: endTime,
        resolution: resolutionSeconds,
        // Several quantiles => the percentile "All" overlay (one series each);
        // otherwise the single picked percentile. Render's `parameters` is a list.
        parameters:
          quantiles && quantiles.length > 0
            ? quantiles.map((q) => ({ quantile: q }))
            : quantile != null
              ? [{ quantile }]
              : undefined,
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
    skipPollAttempt: skipPollWhenHidden,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const unavailable = isMetricsUnavailable(error);
  const storeUnavailable = !unavailable && isLogStoreUnavailable(error);

  // Memoized on data identity: a stable series identity is what lets the
  // charts' geometry useMemos actually cache across poll-tick re-renders.
  const series: ChartSeries[] = useMemo(
    () => toChartSeries(data?.metrics),
    [data],
  );

  const degradedSources = useMemo(() => {
    const joined = series.find((s) => s.labels["degraded_sources"])?.labels[
      "degraded_sources"
    ];
    return joined ? joined.split(",") : [];
  }, [series]);

  return {
    series,
    loading,
    unavailable,
    storeUnavailable,
    error: unavailable || storeUnavailable ? undefined : error,
    degradedSources,
  };
}

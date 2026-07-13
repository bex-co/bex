import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { ChartSeries } from "@/features/metrics/types";

// bex-api's Metrics/DatastoreMetrics verbs report this exact message (Core's
// ErrMetricsUnavailable) when a metric's backend isn't wired (e.g. no
// Prometheus for request metrics) — surfaced here, not as a generic error.
export const METRICS_UNAVAILABLE_MESSAGE = "metrics source not configured";

export function isMetricsUnavailable(error: unknown): boolean {
  return Boolean(
    error &&
    CombinedGraphQLErrors.is(error) &&
    error.errors.some((e) => e.message === METRICS_UNAVAILABLE_MESSAGE),
  );
}

/** The wire shape shared by GraphQL's MetricSeries type — metrics() and
 * datastoreMetrics() both return this, just under different query names. */
export interface RawMetricSeries {
  unit?: string | null;
  labels?: ReadonlyArray<{
    field?: string | null;
    value?: string | null;
  } | null> | null;
  values?: ReadonlyArray<{
    time?: string | null;
    value?: number | null;
  } | null> | null;
}

/** Maps a GraphQL MetricSeries array onto ChartSeries, dropping null/malformed
 * entries — shared by useMetrics and useDatastoreMetrics. */
export function toChartSeries(
  raw: ReadonlyArray<RawMetricSeries | null> | null | undefined,
): ChartSeries[] {
  return (raw ?? [])
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
    }));
}

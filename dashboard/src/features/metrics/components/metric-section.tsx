import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import { MetricError } from "@/features/metrics/components/metric-error";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";
import { CHART_HEIGHT } from "@/features/metrics/components/chart-layout";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

interface MetricSectionProps {
  title: string;
  result: UseMetricsResult;
  /** Rendered beside the title: a control, the latest value, a limit label… */
  headerExtra?: React.ReactNode;
  /** Overrides the generic error-card copy for a section that wants a
   *  metric-specific message (e.g. bandwidth's "Couldn't load bandwidth"). */
  errorMessage?: string;
  children: React.ReactNode;
}

/**
 * One chart section of a metrics card: the shared title row + the
 * source-unavailable branches, so every section on the page keeps identical
 * header typography and 503 handling. Two honest 503 states: no metrics backend
 * ("source not configured") and — for a host/path-filtered request read — no
 * durable log store ("host/path filters need the log store", w5/m58); the
 * filtered chart is never silently unfiltered.
 */
export function MetricSection({
  title,
  result,
  headerExtra,
  errorMessage,
  children,
}: MetricSectionProps) {
  const { t } = useTranslations();
  // While the first fetch is in flight (no series yet), show a chart-shaped
  // shimmer at the exact chart height — NOT the child chart, which would render
  // the "No data in range" empty state and read as broken data on the busiest
  // detail tab (w9/m63 t002). A poll refetch over existing data keeps the chart.
  const loadingEmpty = result.loading && result.series.length === 0;
  // A query that FAILED (not a recognized 503, not still loading) with no series
  // to show gets a distinct error card — before w9/m86 it fell through to the
  // child chart's "No data in range" empty state, indistinguishable from a
  // genuinely empty window (the conflation that hid the w5/m71 bug for months).
  // A refetch error over existing data keeps the chart, mirroring loadingEmpty.
  const errorEmpty = !!result.error && result.series.length === 0;
  const body = result.storeUnavailable ? (
    <MetricUnavailable message={t("metrics.hostPathStoreUnavailable")} />
  ) : result.unavailable ? (
    <MetricUnavailable />
  ) : loadingEmpty ? (
    <Skeleton
      role="status"
      className="w-full rounded-md"
      style={{ height: CHART_HEIGHT }}
      aria-label={t("common.loading")}
    />
  ) : errorEmpty ? (
    <MetricError message={errorMessage} />
  ) : (
    children
  );
  return (
    <div>
      <div className="mb-2 flex items-center justify-between gap-2">
        <div className="text-sm font-medium text-foreground">{title}</div>
        {headerExtra}
      </div>
      {body}
    </div>
  );
}

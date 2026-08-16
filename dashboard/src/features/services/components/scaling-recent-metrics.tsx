import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetricSection } from "@/features/metrics/components/metric-section";
import {
  SvgLineChart,
  EmptyChart,
  type LineSeriesInput,
} from "@/features/metrics/components/svg-line-chart";
import {
  useMetrics,
  type UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { latestValue } from "@/features/metrics/lib/series";
import type { ChartPoint, ChartSeries } from "@/features/metrics/types";
import type { RangeWindow } from "@/features/metrics/lib/range";

// Render's Recent Metrics window is a fixed "past 48 hours" (live capture
// 2026-07-16). 24-minute buckets keep the series at ~120 points, the same
// density the metrics page's presets aim for.
const RECENT_WINDOW: RangeWindow = {
  spanSeconds: 48 * 60 * 60,
  resolutionSeconds: 24 * 60,
};

/**
 * Render's "Recent Metrics" section on the Scaling page (w7/m43): average
 * CPU/memory utilization across all instances plus total instances over the
 * past 48 hours — the data behind an instance-count or autoscaling-target
 * choice, without leaving the page. Reuses the metrics feature's query hook
 * and chart primitives; "View all metrics" links to the full Metrics tab.
 */
export function ScalingRecentMetrics({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  // pollIntervalMs: 0 — the live-range tick is the one refresh schedule;
  // Apollo's default 30s poll would be a second, redundant one re-sending the
  // same frozen bounds (the Metrics tab encodes the same decision).
  const window = { ...useLiveRange(RECENT_WINDOW), pollIntervalMs: 0 };

  const memory = useMetrics(serviceId, "memory", window);
  const memoryLimit = useMetrics(serviceId, "memory_limit", {
    ...window,
    aggregateMax: true,
  });
  const cpu = useMetrics(serviceId, "cpu", window);
  const cpuLimit = useMetrics(serviceId, "cpu_limit", {
    ...window,
    aggregateMax: true,
  });
  const instances = useMetrics(serviceId, "instance_count", window);

  const instancePoints = instances.series[0]?.points ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.scalingMetricsTitle")}</CardTitle>
        <CardDescription className="mt-1">
          {t("services.scalingMetricsNote")}{" "}
          <Link
            to="/services/$serviceId/metrics"
            params={{ serviceId }}
            className="underline underline-offset-2 hover:text-foreground"
          >
            {t("services.scalingMetricsViewAll")}
          </Link>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <AvgUtilizationSection
          title={t("services.scalingMetricsMemory")}
          result={memory}
          limit={latestValue(memoryLimit.series)}
        />
        <AvgUtilizationSection
          title={t("services.scalingMetricsCPU")}
          result={cpu}
          limit={latestValue(cpuLimit.series)}
        />
        <MetricSection title={t("metrics.totalInstances")} result={instances}>
          {instances.loading && instances.series.length === 0 ? (
            <Skeleton className="h-40 w-full" />
          ) : instancePoints.length === 0 ? (
            <EmptyChart message={t("services.scalingMetricsEmpty")} />
          ) : (
            <SvgLineChart
              unit="count"
              series={[{ points: instancePoints, color: "var(--chart-3)" }]}
            />
          )}
        </MetricSection>
      </CardContent>
    </Card>
  );
}

// Averages the per-instance series into one point list, bucketed by
// timestamp — Render's "Across all instances" line. Timestamps within one
// query response share a format, so lexicographic order is chronological.
function averageAcrossInstances(series: ChartSeries[]): ChartPoint[] {
  if (series.length <= 1) return series[0]?.points ?? [];
  const buckets = new Map<string, { sum: number; n: number }>();
  for (const s of series) {
    for (const p of s.points) {
      const entry = buckets.get(p.timestamp) ?? { sum: 0, n: 0 };
      entry.sum += p.value;
      entry.n += 1;
      buckets.set(p.timestamp, entry);
    }
  }
  return [...buckets.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([timestamp, { sum, n }]) => ({ timestamp, value: sum / n }));
}

/**
 * One averaged utilization chart (memory or cpu): the per-instance series
 * averaged into a single line, as % of the App's limit. With no limit
 * configured the percentage is undefined, so the block honestly says so
 * (the metrics page's omit-don't-fake rule) rather than faking a flat line;
 * with no data at all it shows Render's "No data captured…" state.
 */
function AvgUtilizationSection({
  title,
  result,
  limit,
}: {
  title: string;
  result: UseMetricsResult;
  limit: number | null;
}) {
  const { t } = useTranslations();
  const hasData = result.series.some((s) => s.points.length > 0);

  // null = no usable limit ⇒ the percentage is undefined.
  const series = useMemo<LineSeriesInput[] | null>(() => {
    if (limit == null || limit === 0) return null;
    return [
      {
        points: averageAcrossInstances(result.series).map((p) => ({
          ...p,
          value: (p.value / limit) * 100,
        })),
        color: "var(--chart-1)",
      },
    ];
  }, [result.series, limit]);

  // One branch per state: loading / no data / no limit / chart.
  return (
    <MetricSection
      title={title}
      result={result}
      headerExtra={
        <span className="text-xs text-muted-foreground">
          {t("services.scalingMetricsAcross")}
        </span>
      }
    >
      {result.loading && result.series.length === 0 ? (
        <Skeleton className="h-40 w-full" />
      ) : !hasData ? (
        <EmptyChart message={t("services.scalingMetricsEmpty")} />
      ) : series == null ? (
        <EmptyChart message={t("metrics.noLimitConfigured")} />
      ) : (
        <SvgLineChart unit="percentage" series={series} />
      )}
    </MetricSection>
  );
}

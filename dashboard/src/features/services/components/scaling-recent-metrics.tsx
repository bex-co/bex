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

  // Utilization is read as server-side per-instance percentages (w5/m90),
  // whose points keep each pod's own limit history: the current-limit reads
  // below are current-pod values, empty for a sleeping or suspended service,
  // so dividing by them called every such service limitless (w4/192). Those
  // reads only tell a truly limitless App apart now.
  const memory = useMetrics(serviceId, "memory", window);
  const memoryPercentage = useMetrics(serviceId, "memory", {
    ...window,
    percentage: true,
  });
  const memoryLimit = useMetrics(serviceId, "memory_limit", {
    ...window,
    aggregateMax: true,
  });
  const cpu = useMetrics(serviceId, "cpu", window);
  const cpuPercentage = useMetrics(serviceId, "cpu", {
    ...window,
    percentage: true,
  });
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
          usage={memory}
          percentage={memoryPercentage}
          limit={latestValue(memoryLimit.series)}
        />
        <AvgUtilizationSection
          title={t("services.scalingMetricsCPU")}
          usage={cpu}
          percentage={cpuPercentage}
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
 * One averaged utilization chart (memory or cpu): the server's per-instance
 * percentage series averaged into a single line. With usage but no surviving
 * percentage point, the block says why — no limit configured at all, or a
 * limit history the percentage could not be computed from — rather than
 * faking a line (the Metrics tab's omit-don't-fake rule); with no usage at all
 * it shows Render's "No data captured…" state.
 */
function AvgUtilizationSection({
  title,
  usage,
  percentage,
  limit,
}: {
  title: string;
  usage: UseMetricsResult;
  percentage: UseMetricsResult;
  limit: number | null;
}) {
  const { t } = useTranslations();
  const hasUsage = usage.series.some((s) => s.points.length > 0);
  const hasPercentage = percentage.series.some((s) => s.points.length > 0);

  const series = useMemo<LineSeriesInput[]>(
    () => [
      {
        points: averageAcrossInstances(percentage.series),
        color: "var(--chart-1)",
      },
    ],
    [percentage.series],
  );

  const loading =
    (usage.loading && usage.series.length === 0) ||
    (percentage.loading && percentage.series.length === 0);

  // One branch per state: loading / no data / no limit / unavailable / chart.
  return (
    <MetricSection
      title={title}
      result={percentage}
      headerExtra={
        <span className="text-xs text-muted-foreground">
          {t("services.scalingMetricsAcross")}
        </span>
      }
    >
      {loading ? (
        <Skeleton className="h-40 w-full" />
      ) : hasPercentage ? (
        <SvgLineChart unit="percentage" series={series} />
      ) : !hasUsage ? (
        <EmptyChart message={t("services.scalingMetricsEmpty")} />
      ) : limit == null || limit === 0 ? (
        <EmptyChart message={t("metrics.noLimitConfigured")} />
      ) : (
        <EmptyChart message={t("metrics.percentageUnavailable")} />
      )}
    </MetricSection>
  );
}

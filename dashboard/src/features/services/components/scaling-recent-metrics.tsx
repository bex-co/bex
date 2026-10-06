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
import {
  summarizeLimits,
  type LimitSummary,
} from "@/features/metrics/lib/limit-summary";
import { hasPoints } from "@/features/metrics/lib/series";
import { utilizationEmptyReason } from "@/features/metrics/lib/utilization";
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
export function ScalingRecentMetrics({
  serviceId,
  runsInstances,
}: {
  serviceId: string;
  /** The service runs instances now: not suspended or asleep. */
  runsInstances: boolean;
}) {
  const { t } = useTranslations();
  // pollIntervalMs: 0 — the live-range tick is the one refresh schedule;
  // Apollo's default 30s poll would be a second, redundant one re-sending the
  // same frozen bounds (the Metrics tab encodes the same decision).
  const window = { ...useLiveRange(RECENT_WINDOW), pollIntervalMs: 0 };

  // Utilization is read as server-side per-instance percentages (w5/m90),
  // whose points keep each pod's own limit history. The absolute and
  // current-limit reads only say why a chart is empty (utilizationEmptyReason):
  // the percentage read alone cannot tell no usage from a lost limit history.
  const memory = useMetrics(serviceId, "memory", window);
  const memoryPercentage = useMetrics(serviceId, "memory", {
    ...window,
    percentage: true,
  });
  // Only whether a limit exists is read, and only while instances run.
  const memoryLimit = useMetrics(serviceId, "memory_limit", {
    ...window,
    skip: !runsInstances,
  });
  const cpu = useMetrics(serviceId, "cpu", window);
  const cpuPercentage = useMetrics(serviceId, "cpu", {
    ...window,
    percentage: true,
  });
  const cpuLimit = useMetrics(serviceId, "cpu_limit", {
    ...window,
    skip: !runsInstances,
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
          limit={summarizeLimits(memoryLimit.series)}
          runsInstances={runsInstances}
        />
        <AvgUtilizationSection
          title={t("services.scalingMetricsCPU")}
          usage={cpu}
          percentage={cpuPercentage}
          limit={summarizeLimits(cpuLimit.series)}
          runsInstances={runsInstances}
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
 * percentage series averaged into a single line. With no line, it says why,
 * as the Metrics tab does (utilizationEmptyReason): no usage captured, no
 * limit configured, or percentages unavailable.
 */
function AvgUtilizationSection({
  title,
  usage,
  percentage,
  limit,
  runsInstances,
}: {
  title: string;
  usage: UseMetricsResult;
  percentage: UseMetricsResult;
  limit: LimitSummary;
  runsInstances: boolean;
}) {
  const { t } = useTranslations();
  const reason = utilizationEmptyReason({
    percentage: hasPoints(percentage.series),
    usage: hasPoints(usage.series),
    limit,
    runsInstances,
  });

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
      ) : reason === null ? (
        <SvgLineChart unit="percentage" series={series} />
      ) : reason === "no-usage" ? (
        <EmptyChart message={t("services.scalingMetricsEmpty")} />
      ) : reason === "no-limit" ? (
        <EmptyChart message={t("metrics.noLimitConfigured")} />
      ) : (
        <EmptyChart message={t("metrics.percentageUnavailable")} />
      )}
    </MetricSection>
  );
}

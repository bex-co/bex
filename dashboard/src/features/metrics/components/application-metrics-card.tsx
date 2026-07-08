import { useMemo } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  SvgLineChart,
  ChartLegend,
  EmptyChart,
  type LineSeriesInput,
} from "@/features/metrics/components/svg-line-chart";
import { MetricSection } from "@/features/metrics/components/metric-section";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import {
  useMetrics,
  type UseMetricsOptions,
  type UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMetricValue } from "@/features/metrics/lib/format";

interface ApplicationMetricsCardProps {
  resource: string;
  percentage: boolean;
  /** The page's resolved live window (route-owned, shared with the network card). */
  window: UseMetricsOptions;
}

/**
 * Render's "Application Metrics" card: Memory, CPU, Total Instances as
 * stepped history charts over the selected range — one line per instance
 * (cAdvisor via Prometheus, or a single current point on the metrics-server
 * fallback; docs/observability.md).
 *
 * Percentage/Total is computed client-side from two already-fetched series
 * (the raw metric + its _limit counterpart, aggregated to one max value) —
 * captured live from Render's dashboard: it fetches both regardless of which
 * tab is selected rather than asking the backend for a percentage. The same
 * limit feeds the "Limit …" header label and the Total tab's dashed
 * reference line. The limit queries ride the same live-window tick as the
 * usage queries (no second Apollo poll timer — limits only change on
 * redeploy anyway).
 */
export function ApplicationMetricsCard({
  resource,
  percentage,
  window,
}: ApplicationMetricsCardProps) {
  const { t } = useTranslations();
  const cpu = useMetrics(resource, "cpu", window);
  const memory = useMetrics(resource, "memory", window);
  const cpuLimit = useMetrics(resource, "cpu_limit", {
    ...window,
    aggregateMax: true,
  });
  const memoryLimit = useMetrics(resource, "memory_limit", {
    ...window,
    aggregateMax: true,
  });
  const instances = useMetrics(resource, "instance_count", window);

  const instancesSeries = useMemo<LineSeriesInput[]>(
    () => [
      { points: instances.series[0]?.points ?? [], color: "var(--chart-3)" },
    ],
    [instances.series],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("metrics.applicationTitle")}</CardTitle>
        <CardDescription>{t("metrics.applicationDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <ResourceSection
          title={t("metrics.memory")}
          result={memory}
          limit={latestValue(memoryLimit.series)}
          limitUnit="bytes"
          percentage={percentage}
        />
        <ResourceSection
          title={t("metrics.cpu")}
          result={cpu}
          limit={latestValue(cpuLimit.series)}
          limitUnit="cpu"
          percentage={percentage}
        />
        <MetricSection
          title={t("metrics.totalInstances")}
          result={instances}
          headerExtra={
            <LatestValue result={instances} unit="count" value={latestValue(instances.series)} />
          }
        >
          <SvgLineChart unit="count" series={instancesSeries} />
        </MetricSection>
      </CardContent>
    </Card>
  );
}

interface ResourceSectionProps {
  title: string;
  result: UseMetricsResult;
  /** The App's limit (max across instances) — null when none is configured. */
  limit: number | null;
  limitUnit: string;
  percentage: boolean;
}

/**
 * One usage chart (memory or cpu): per-instance lines, latest value + limit
 * in the header. Percentage mode divides every point by the limit; with no
 * limit configured that division is undefined, so the chart honestly says so
 * instead of faking a flat line (same omit-don't-fake rule as bex-api).
 */
function ResourceSection({
  title,
  result,
  limit,
  limitUnit,
  percentage,
}: ResourceSectionProps) {
  const { t } = useTranslations();

  const hasLimit = limit != null && limit !== 0;
  const noLimit = percentage && !hasLimit;
  const unit = percentage ? "percentage" : (result.series[0]?.unit ?? limitUnit);

  const series = useMemo<LineSeriesInput[]>(
    () =>
      noLimit
        ? []
        : result.series.map((s, i) => ({
            points: percentage
              ? s.points.map((p) => ({
                  ...p,
                  value: (p.value / limit!) * 100,
                }))
              : s.points,
            color: seriesColor(i),
            label: s.labels["instance"],
          })),
    [result.series, percentage, limit, noLimit],
  );

  return (
    <MetricSection
      title={title}
      result={result}
      headerExtra={
        <div className="flex items-baseline gap-2">
          {hasLimit && (
            <span className="text-xs text-muted-foreground">
              {t("metrics.limitLabel", {
                value: formatMetricValue(limitUnit, limit),
              })}
            </span>
          )}
          {!noLimit && (
            <LatestValue
              result={result}
              unit={unit}
              value={latestValue(series)}
            />
          )}
        </div>
      }
    >
      {noLimit ? (
        <EmptyChart message={t("metrics.noLimitConfigured")} />
      ) : (
        <>
          <SvgLineChart
            unit={unit}
            series={series}
            referenceValue={!percentage && hasLimit ? limit : undefined}
          />
          <ChartLegend entries={series} />
        </>
      )}
    </MetricSection>
  );
}

/** The latest observed value beside a section title ("…" while first loading). */
function LatestValue({
  result,
  unit,
  value,
}: {
  result: UseMetricsResult;
  unit: string;
  value: number | null;
}) {
  const text =
    result.loading && result.series.length === 0
      ? "…"
      : value != null
        ? formatMetricValue(unit, value)
        : null;
  if (text == null) return null;
  return (
    <span className="text-sm font-semibold tabular-nums text-foreground">
      {text}
    </span>
  );
}

/** The last point's value of the first series — null when there is none. */
function latestValue(series: { points: { value: number }[] }[]): number | null {
  const points = series[0]?.points ?? [];
  return points.length > 0 ? points[points.length - 1].value : null;
}

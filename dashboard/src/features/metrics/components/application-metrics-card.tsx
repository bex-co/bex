import { useMemo, useState, useEffect } from "react";
import { Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs.tsx";
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
import { useMetricsFilterValues } from "@/features/metrics/hooks/use-metrics-filter-values";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMetricValue } from "@/features/metrics/lib/format";
import { latestValue } from "@/features/metrics/lib/series";
import type { ChartEventMarker } from "@/features/metrics/lib/chart-events";

interface ApplicationMetricsCardProps {
  /** The service id — names the metrics resource and the plan/scaling links. */
  resource: string;
  /** The page's resolved live window (route-owned, shared with the network card). */
  window: UseMetricsOptions;
  /** Service events in the window, marked on every chart (route-derived). */
  markers?: ChartEventMarker[];
}

/**
 * Render's "Application Metrics" card: Memory, CPU, Total Instances as
 * stepped history charts over the selected range — one line per instance
 * (cAdvisor via Prometheus, or a single current point on the metrics-server
 * fallback; docs/ADR010-observability.md). The Percentage/Total tabs sit in
 * the card header (Render's placement, captured live 2026-07-17, w5/m42) —
 * they alter only this card, so their state lives here, not on the page.
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
  window,
  markers,
}: ApplicationMetricsCardProps) {
  const { t } = useTranslations();
  const [percentage, setPercentage] = useState(true); // Render defaults to Percentage
  // Raw = per-instance series; MIN/MAX/AVG are backend replica aggregates (w5/m89).
  const [aggregateMethod, setAggregateMethod] = useState<
    "" | "MIN" | "MAX" | "AVG"
  >("");
  const [selectedInstances, setSelectedInstances] = useState<string[]>([]);

  const liveInstances = useMetricsFilterValues(resource, "INSTANCE");
  // Unfiltered CPU series seeds historical instance choices (terminated pods
  // still in the window). Selection never silently broadens when a choice
  // leaves — see the prune effect below.
  const inventory = useMetrics(resource, "cpu", window);
  const resourceOpts: UseMetricsOptions = {
    ...window,
    ...(selectedInstances.length > 0
      ? { instances: selectedInstances }
      : {}),
    ...(aggregateMethod ? { aggregateMethod } : {}),
  };
  const cpu = useMetrics(resource, "cpu", resourceOpts);
  const memory = useMetrics(resource, "memory", resourceOpts);
  const cpuLimit = useMetrics(resource, "cpu_limit", {
    ...resourceOpts,
    aggregateMax: true,
    aggregateMethod: aggregateMethod || undefined,
  });
  const memoryLimit = useMetrics(resource, "memory_limit", {
    ...resourceOpts,
    aggregateMax: true,
    aggregateMethod: aggregateMethod || undefined,
  });
  // Autoscale target (w3/m10, w1/m20's config): a single current-value point,
  // omitted server-side when autoscaling is disabled — latestValue then
  // naturally resolves to null and the target line/label just don't render.
  const cpuTarget = useMetrics(resource, "cpu_target", window);
  const memoryTarget = useMetrics(resource, "memory_target", window);
  const instances = useMetrics(resource, "instance_count", window);

  // Live discovery ∪ labels already in the window (terminated replicas).
  const instanceChoices = useMemo(() => {
    const fromSeries = new Set<string>();
    for (const s of inventory.series) {
      const id = s.labels["instance"];
      if (id) fromSeries.add(id);
    }
    return Array.from(new Set([...liveInstances, ...fromSeries])).sort();
  }, [liveInstances, inventory.series]);

  // Drop selections that left the window rather than silently selecting all.
  useEffect(() => {
    setSelectedInstances((prev) => {
      const next = prev.filter((id) => instanceChoices.includes(id));
      return next.length === prev.length ? prev : next;
    });
  }, [instanceChoices]);

  const instancesSeries = useMemo<LineSeriesInput[]>(
    () => [
      { points: instances.series[0]?.points ?? [], color: "var(--chart-3)" },
    ],
    [instances.series],
  );

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-center justify-between gap-3">
        <CardTitle>{t("metrics.applicationTitle")}</CardTitle>
        <div className="flex flex-wrap items-center gap-2">
          {instanceChoices.length > 0 ? (
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="sr-only">{t("metrics.instanceFilter")}</span>
              <select
                multiple
                aria-label={t("metrics.instanceFilter")}
                className="h-9 min-w-[9rem] max-w-[14rem] rounded-md border bg-background px-2 text-xs text-foreground"
                value={selectedInstances}
                onChange={(e) => {
                  const next = Array.from(e.target.selectedOptions).map(
                    (o) => o.value,
                  );
                  setSelectedInstances(next);
                }}
              >
                {instanceChoices.map((id) => (
                  <option key={id} value={id}>
                    {shortInstanceLabel(id)}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          <Tabs
            value={aggregateMethod || "raw"}
            onValueChange={(v) =>
              setAggregateMethod(
                v === "raw" ? "" : (v as "MIN" | "MAX" | "AVG"),
              )
            }
          >
            <TabsList aria-label={t("metrics.aggregateFilter")}>
              <TabsTrigger value="raw">{t("metrics.aggregateRaw")}</TabsTrigger>
              <TabsTrigger value="MIN">{t("metrics.aggregateMin")}</TabsTrigger>
              <TabsTrigger value="MAX">{t("metrics.aggregateMax")}</TabsTrigger>
              <TabsTrigger value="AVG">{t("metrics.aggregateAvg")}</TabsTrigger>
            </TabsList>
          </Tabs>
          <Tabs
            value={percentage ? "percentage" : "total"}
            onValueChange={(v) => setPercentage(v === "percentage")}
          >
            <TabsList>
              <TabsTrigger value="percentage">
                {t("metrics.filterPercentage")}
              </TabsTrigger>
              <TabsTrigger value="total">{t("metrics.filterTotal")}</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <ResourceSection
          title={t("metrics.memory")}
          serviceId={resource}
          result={memory}
          limit={latestValue(memoryLimit.series)}
          limitUnit="bytes"
          target={latestValue(memoryTarget.series)}
          percentage={percentage}
          markers={markers}
        />
        <ResourceSection
          title={t("metrics.cpu")}
          serviceId={resource}
          result={cpu}
          limit={latestValue(cpuLimit.series)}
          limitUnit="cpu"
          target={latestValue(cpuTarget.series)}
          percentage={percentage}
          markers={markers}
        />
        <MetricSection
          title={t("metrics.totalInstances")}
          result={instances}
          headerExtra={
            <div className="flex items-baseline gap-2">
              <Link
                to="/services/$serviceId/scaling"
                params={{ serviceId: resource }}
                className="text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
              >
                {t("metrics.manageScaling")}
              </Link>
              <LatestValue
                result={instances}
                unit="count"
                value={latestValue(instances.series)}
              />
            </div>
          }
        >
          <SvgLineChart
            unit="count"
            series={instancesSeries}
            markers={markers}
            markersServiceId={resource}
          />
        </MetricSection>
      </CardContent>
    </Card>
  );
}

/** Shorten opaque instance suffixes for the selector without changing the value. */
function shortInstanceLabel(id: string): string {
  const parts = id.split("-");
  const suffix = parts[parts.length - 1] ?? id;
  if (/^[0-9a-v]{20}$/.test(suffix) && suffix.length > 5) {
    return `${parts.slice(0, -1).join("-")}-${suffix.slice(0, 5)}`;
  }
  return id;
}

interface ResourceSectionProps {
  title: string;
  /** Names the header's `Limit` link target (the service's Instance Type tab). */
  serviceId: string;
  result: UseMetricsResult;
  /** The App's limit (max across instances) — null when none is configured. */
  limit: number | null;
  limitUnit: string;
  /**
   * The App's configured autoscale-target utilization (0..100), null when
   * autoscaling is disabled/unconfigured (w3/m10, w1/m20). Only meaningful in
   * percentage mode — both target and the %-of-limit chart share the same
   * 0..100 scale, so it overlays as the dashed reference line there (the
   * limit line takes that slot in absolute mode instead).
   */
  target?: number | null;
  percentage: boolean;
  markers?: ChartEventMarker[];
}

/**
 * One usage chart (memory or cpu): per-instance lines, latest value + limit
 * in the header. Percentage mode divides every point by the limit; with no
 * limit configured that division is undefined, so the chart honestly says so
 * instead of faking a flat line (same omit-don't-fake rule as bex-api).
 */
function ResourceSection({
  title,
  serviceId,
  result,
  limit,
  limitUnit,
  target,
  percentage,
  markers,
}: ResourceSectionProps) {
  const { t } = useTranslations();

  const hasLimit = limit != null && limit !== 0;
  const hasTarget = target != null;
  const noLimit = percentage && !hasLimit;
  const unit = percentage
    ? "percentage"
    : (result.series[0]?.unit ?? limitUnit);

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
          <span className="text-xs text-muted-foreground">
            {/* Render links "Limit" to the plan page — with no limit set, the
                link still leads where one is configured (w5/m42). */}
            <Link
              to="/services/$serviceId/plan"
              params={{ serviceId }}
              className="underline underline-offset-2 hover:text-foreground"
            >
              {t("metrics.limitLink")}
            </Link>
            {hasLimit && <> {formatMetricValue(limitUnit, limit)}</>}
          </span>
          {percentage && hasTarget && (
            <span className="text-xs text-muted-foreground">
              {t("metrics.targetLabel", {
                value: formatMetricValue("percentage", target),
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
            referenceValue={
              percentage
                ? hasTarget
                  ? target
                  : undefined
                : hasLimit
                  ? limit
                  : undefined
            }
            markers={markers}
            markersServiceId={serviceId}
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

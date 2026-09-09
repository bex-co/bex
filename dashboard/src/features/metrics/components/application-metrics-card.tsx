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
import type { ChartSeries } from "@/features/metrics/types";

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
 * Percentage is a server-side read (bex-api `percentage`, w5/m90): each
 * replica normalized by its own trustworthy limit before any MIN/MAX/AVG
 * aggregation — never the old client-side division of every point by one
 * latest aggregate limit, which misreported mixed-limit replicas (0.4 of 0.5
 * read as 40%). Total renders the absolute usage series untouched. The
 * per-instance _limit series feed only the header label and the Total tab's
 * dashed reference line, so mixed limits read as "Limits vary" instead of
 * one max presented as universal.
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
  // Absolute usage (Total tab; also the "is there anything to percentize?"
  // witness for the percentage-unavailable state).
  const cpuAbsolute = useMetrics(resource, "cpu", resourceOpts);
  const memoryAbsolute = useMetrics(resource, "memory", resourceOpts);
  // Server-side per-instance percentages (w5/m90) — rendered as returned.
  const cpuPercentage = useMetrics(resource, "cpu", {
    ...resourceOpts,
    percentage: true,
  });
  const memoryPercentage = useMetrics(resource, "memory", {
    ...resourceOpts,
    percentage: true,
  });
  // Per-instance limits for truthful headers (w5/m90): no aggregate collapse,
  // so a uniform limit still reads as one value while mixed limits read as
  // "Limits vary". Scoped to the selection, like the usage queries.
  const limitOpts: UseMetricsOptions = {
    ...window,
    ...(selectedInstances.length > 0
      ? { instances: selectedInstances }
      : {}),
  };
  const cpuLimit = useMetrics(resource, "cpu_limit", limitOpts);
  const memoryLimit = useMetrics(resource, "memory_limit", limitOpts);
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
          result={percentage ? memoryPercentage : memoryAbsolute}
          absolute={memoryAbsolute}
          limit={summarizeLimits(memoryLimit.series)}
          limitUnit="bytes"
          target={latestValue(memoryTarget.series)}
          percentage={percentage}
          markers={markers}
        />
        <ResourceSection
          title={t("metrics.cpu")}
          serviceId={resource}
          result={percentage ? cpuPercentage : cpuAbsolute}
          absolute={cpuAbsolute}
          limit={summarizeLimits(cpuLimit.series)}
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

/**
 * One App limit as the header can honestly state it: no limit series at all,
 * one uniform value across the selected replicas, or mixed per-replica
 * limits. Computed from the per-instance _limit series' latest points.
 */
export type LimitSummary =
  | { kind: "none" }
  | { kind: "single"; value: number }
  | { kind: "vary" };

export function summarizeLimits(series: ChartSeries[]): LimitSummary {
  const values = series
    .map((s) =>
      s.points.length > 0 ? s.points[s.points.length - 1].value : null,
    )
    .filter((v): v is number => v != null);
  if (values.length === 0) return { kind: "none" };
  return values.every((v) => v === values[0])
    ? { kind: "single", value: values[0] }
    : { kind: "vary" };
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
  /** The series to chart: server percentages in Percentage mode, absolutes in Total. */
  result: UseMetricsResult;
  /** The absolute usage read for the same selection — the witness that tells
   *  "no trustworthy limit" apart from "no usage samples". */
  absolute: UseMetricsResult;
  /** The App's limit across the selected replicas — null when none is configured. */
  limit: LimitSummary;
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
 * in the header. In Percentage mode the series arrive already normalized by
 * bex-api (each replica against its own trustworthy limit); samples without
 * a trustworthy denominator are omitted server-side, so an empty percentage
 * chart over non-empty usage honestly says percentages are unavailable
 * instead of faking a flat line. Mixed per-replica limits read as
 * "Limits vary", never as one max presented as universal.
 */
function ResourceSection({
  title,
  serviceId,
  result,
  absolute,
  limit,
  limitUnit,
  target,
  percentage,
  markers,
}: ResourceSectionProps) {
  const { t } = useTranslations();

  const hasTarget = target != null;
  const absoluteHasData =
    absolute.series.some((s) => s.points.length > 0) ||
    result.series.some((s) => s.points.length > 0);
  // Percentage over observed usage but no percentage point survived: every
  // denominator was missing, zero, or otherwise untrustworthy (deleted pods,
  // predated limit retention, a mid-window rollout gap) — distinct from "no
  // usage samples" and from a source failure (both handled elsewhere).
  const percentagesUnavailable =
    percentage &&
    result.series.every((s) => s.points.length === 0) &&
    absolute.series.some((s) => s.points.length > 0);
  // No limit configured at all (and usage observed): the division is
  // undefined, so the chart honestly says so instead of faking a flat line
  // (same omit-don't-fake rule as bex-api).
  const noLimit =
    percentage && limit.kind === "none" && absoluteHasData;
  const unit = percentage
    ? "percentage"
    : (result.series[0]?.unit ?? limitUnit);

  const series = useMemo<LineSeriesInput[]>(
    () =>
      result.series.map((s, i) => ({
        points: s.points,
        color: seriesColor(i),
        label: s.labels["instance"],
      })),
    [result.series],
  );

  // The Total tab's dashed reference line is only truthful for a uniform
  // limit; mixed limits have no one applicable value, so the line is omitted
  // and the header says so.
  const referenceValue = percentage
    ? (hasTarget ? target : undefined)
    : limit.kind === "single"
      ? limit.value
      : undefined;

  return (
    <MetricSection
      title={title}
      result={
        percentage && absolute.loading
          ? { ...result, loading: true }
          : result
      }
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
            {limit.kind === "single" && (
              <> {formatMetricValue(limitUnit, limit.value)}</>
            )}
            {limit.kind === "vary" && <> {t("metrics.limitsVary")}</>}
          </span>
          {percentage && hasTarget && (
            <span className="text-xs text-muted-foreground">
              {t("metrics.targetLabel", {
                value: formatMetricValue("percentage", target),
              })}
            </span>
          )}
          {!noLimit && !percentagesUnavailable && (
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
      ) : percentagesUnavailable ? (
        <EmptyChart message={t("metrics.percentageUnavailable")} />
      ) : (
        <>
          <SvgLineChart
            unit={unit}
            series={series}
            referenceValue={referenceValue}
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

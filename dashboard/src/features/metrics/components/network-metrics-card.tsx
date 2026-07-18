import { useMemo, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import {
  SvgBarChart,
  type BarSeriesInput,
} from "@/features/metrics/components/svg-bar-chart";
import {
  SvgLineChart,
  ChartLegend,
  type LineSeriesInput,
} from "@/features/metrics/components/svg-line-chart";
import { MetricSection } from "@/features/metrics/components/metric-section";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import {
  useMetrics,
  type UseMetricsOptions,
} from "@/features/metrics/hooks/use-metrics";
import { useMetricsFilterValues } from "@/features/metrics/hooks/use-metrics-filter-values";
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMegabytes } from "@/features/metrics/lib/format";

interface NetworkMetricsCardProps {
  resource: string;
  /** The page's resolved live window (route-owned, shared with the app card). */
  window: UseMetricsOptions;
}

type GroupBy = "status" | "method" | undefined;
const GROUP_BY_ALL = "all";

// Render's Percentile options on Response Times (captured live 2026-07-17,
// w5/m42; its "All" overlay needs multi-quantile queries bex-api doesn't
// serve — accepted drift). p50/p90/p99 are quantile notation, not English
// words — no translation needed.
const PERCENTILES = [
  { value: "0.5", label: "p50" },
  { value: "0.75", label: "p75" },
  { value: "0.9", label: "p90" },
  { value: "0.99", label: "p99" },
];
const DEFAULT_QUANTILE = 0.9; // Render defaults to p90

// Render's Status Code dropdown offers the class presets plus the codes the
// App has actually returned (discovered via metricsFilters). "all" is the
// no-filter sentinel — Radix Select can't represent an empty-string value.
const STATUS_CODE_ALL = "all";
const STATUS_CODE_CLASSES = ["2xx", "4xx", "5xx"];

/**
 * Render's "Network Metrics" card: Total Requests, Response Times, Outbound
 * Bandwidth — Prometheus-backed time-series honoring the range/resolution.
 * The Status Code filter sits in the card header and the Percentile picker on
 * the Response Times section (Render's placement, captured live 2026-07-17,
 * w5/m42) — both alter only this card, so their state lives here. Status Code
 * applies to requests + latency; bandwidth is deliberately left unfiltered
 * because its composed HTTP, WebSocket, and direct-public sources do not share
 * a status-code label (same honesty rule as the backend's rejected host/path
 * filters — w3/m12).
 */
export function NetworkMetricsCard({
  resource,
  window,
}: NetworkMetricsCardProps) {
  const { t } = useTranslations();
  // Render puts Group by on the Total Requests chart itself (not the page
  // toolbar), so its state lives here with the chart.
  const [groupBy, setGroupBy] = useState<GroupBy>(undefined);
  const [quantile, setQuantile] = useState(DEFAULT_QUANTILE);
  const [statusCode, setStatusCode] = useState(""); // "" = all
  const discoveredStatusCodes = useMetricsFilterValues(resource, "STATUS_CODE");
  const queryOpts = { ...window, statusCode: statusCode || undefined };
  const requests = useMetrics(resource, "http_requests", {
    ...queryOpts,
    groupBy,
  });
  const latency = useMetrics(resource, "http_latency", {
    ...queryOpts,
    quantile,
  });
  const bandwidth = useMetrics(resource, "bandwidth", window);
  const monthToDate = useMonthToDateBandwidth(resource);

  const statusCodeOptions = [
    ...STATUS_CODE_CLASSES,
    ...discoveredStatusCodes
      .filter((c) => !STATUS_CODE_CLASSES.includes(c))
      .sort(),
  ];

  const requestSeries = useMemo<BarSeriesInput[]>(
    () =>
      requests.series.map((s, i) => ({
        points: s.points,
        color: seriesColor(i, 3), // this chart's base hue is chart-4
        label: groupBy ? groupLabel(s.labels) : undefined,
      })),
    [requests.series, groupBy],
  );
  // Render titles the section with the window's aggregate ("7,266 requests").
  const requestCount = useMemo(
    () =>
      Math.round(
        requests.series.reduce(
          (sum, s) => s.points.reduce((acc, p) => acc + p.value, sum),
          0,
        ),
      ),
    [requests.series],
  );
  const latencySeries = useMemo<LineSeriesInput[]>(
    () => [
      { points: latency.series[0]?.points ?? [], color: "var(--chart-5)" },
    ],
    [latency.series],
  );
  const bandwidthSeries = useMemo<LineSeriesInput[]>(
    () => [
      { points: bandwidth.series[0]?.points ?? [], color: "var(--chart-1)" },
    ],
    [bandwidth.series],
  );

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-2">
        <div className="space-y-1.5">
          <CardTitle>{t("metrics.networkTitle")}</CardTitle>
          <CardDescription>{t("metrics.networkDescription")}</CardDescription>
        </div>
        <Select
          value={statusCode === "" ? STATUS_CODE_ALL : statusCode}
          onValueChange={(v) => setStatusCode(v === STATUS_CODE_ALL ? "" : v)}
        >
          <SelectTrigger
            size="sm"
            className="w-40"
            aria-label={t("metrics.statusCode")}
          >
            <span className="text-muted-foreground">
              {t("metrics.statusCode")}
            </span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={STATUS_CODE_ALL}>
              {t("metrics.statusCodeAll")}
            </SelectItem>
            {statusCodeOptions.map((code) => (
              <SelectItem key={code} value={code}>
                {code}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection
          title={t("metrics.totalRequests")}
          result={requests}
          headerExtra={
            <div className="flex items-center gap-2">
              {requestCount > 0 && (
                <span className="text-xs text-muted-foreground">
                  {t("metrics.requestsCount", {
                    count: requestCount.toLocaleString(),
                  })}
                </span>
              )}
              <Select
                value={groupBy ?? GROUP_BY_ALL}
                onValueChange={(v) =>
                  setGroupBy(v === GROUP_BY_ALL ? undefined : (v as GroupBy))
                }
              >
                <SelectTrigger
                  size="sm"
                  className="h-7 w-40"
                  aria-label={t("metrics.groupBy")}
                >
                  <span className="text-muted-foreground">
                    {t("metrics.groupBy")}
                  </span>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={GROUP_BY_ALL}>
                    {t("metrics.groupByAllRequests")}
                  </SelectItem>
                  <SelectItem value="status">
                    {t("metrics.groupByStatus")}
                  </SelectItem>
                  <SelectItem value="method">
                    {t("metrics.groupByMethod")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          }
        >
          <SvgBarChart
            unit={requests.series[0]?.unit ?? "count"}
            series={requestSeries}
          />
          <ChartLegend entries={requestSeries} />
        </MetricSection>
        <MetricSection
          title={t("metrics.responseTimes")}
          result={latency}
          headerExtra={
            <Select
              value={String(quantile)}
              onValueChange={(v) => setQuantile(Number(v))}
            >
              <SelectTrigger
                size="sm"
                className="h-7 w-36"
                aria-label={t("metrics.percentile")}
              >
                <span className="text-muted-foreground">
                  {t("metrics.percentile")}
                </span>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PERCENTILES.map((q) => (
                  <SelectItem key={q.value} value={q.value}>
                    {q.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          }
        >
          <SvgLineChart
            unit={latency.series[0]?.unit ?? "seconds"}
            series={latencySeries}
          />
        </MetricSection>
        <MetricSection
          title={t("metrics.outboundBandwidth")}
          result={bandwidth}
        >
          <SvgLineChart
            unit={bandwidth.series[0]?.unit ?? "bytes"}
            series={bandwidthSeries}
          />
        </MetricSection>
        {monthToDate.egressBandwidthMB != null && (
          <p className="text-sm text-muted-foreground">
            {t("metrics.monthToDateBandwidth", {
              amount: formatMegabytes(monthToDate.egressBandwidthMB),
            })}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

/** The grouped series' display name: its status code or method label. */
function groupLabel(labels: Record<string, string>): string {
  return labels["code"] ?? labels["method"] ?? "";
}

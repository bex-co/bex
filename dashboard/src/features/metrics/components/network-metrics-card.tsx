import { useMemo, useState } from "react";
import { AlertTriangle, X } from "lucide-react";
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
import { Input } from "@/common/components/ui/input.tsx";
import { useLogLabelValues } from "@/features/logs/hooks/use-log-label-values";
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
import type { ChartEventMarker } from "@/features/metrics/lib/chart-events";

interface NetworkMetricsCardProps {
  resource: string;
  /** The page's resolved live window (route-owned, shared with the app card). */
  window: UseMetricsOptions;
  /** Service events in the window, marked on every chart (route-derived). */
  markers?: ChartEventMarker[];
}

type GroupBy = "status" | "method" | undefined;
const GROUP_BY_ALL = "all";

// Render's Percentile options on Response Times (captured live 2026-07-17,
// w5/m42), plus its "All" overlay (w5/m56): bex-api now serves multi-quantile
// reads, so "All" fetches p50/p90/p99 in one query and overlays them. p50/p90/
// p99 are quantile notation, not English words — no translation needed.
const PERCENTILES = [
  { value: "0.5", label: "p50" },
  { value: "0.75", label: "p75" },
  { value: "0.9", label: "p90" },
  { value: "0.99", label: "p99" },
];
const DEFAULT_QUANTILE = 0.9; // Render defaults to p90
// The percentile "All" overlay reads these three at once (Render's p50/p90/p99).
const PERCENTILE_ALL = "all";
const ALL_QUANTILES = [0.5, 0.9, 0.99];

// Render's Status Code dropdown offers the class presets plus the codes the
// App has actually returned (discovered via metricsFilters). "all" is the
// no-filter sentinel — Radix Select can't represent an empty-string value.
const STATUS_CODE_ALL = "all";
const STATUS_CODE_CLASSES = ["2xx", "4xx", "5xx"];

// Render's Host filter (w5/m58). Discovered from the App's request-log hosts via
// the logs label-values read — the same source the Logs tab uses — so it lists
// only hosts the App actually serves. "all" is the no-filter sentinel. Path has
// no dropdown: `path` is a high-cardinality line field, not a discoverable
// label, so (like the Logs tab) it is a free-text filter, not a fabricated list.
const HOST_ALL = "all";

/**
 * Render's "Network Metrics" card: Total Requests, Response Times, Outbound
 * Bandwidth — Prometheus-backed time-series honoring the range/resolution.
 * The Status Code, Host, and Path filters sit in the card header and the
 * Percentile picker on the Response Times section (Render's placement, captured
 * live 2026-07-17, w5/m42; Host/Path added w5/m58) — all alter only this card,
 * so their state lives here. Status Code / Host / Path apply to requests +
 * latency; bandwidth is deliberately left unfiltered because its composed HTTP,
 * WebSocket, and direct-public sources share no such label. Host/Path are served
 * from the request-log store (Loki) — the only backend with a per-request
 * host/path axis; with no store wired those two sections show an explicit
 * "needs the log store" state (via MetricSection), never a silent unfiltered
 * chart.
 */
export function NetworkMetricsCard({
  resource,
  window,
  markers,
}: NetworkMetricsCardProps) {
  const { t } = useTranslations();
  // Render puts Group by on the Total Requests chart itself (not the page
  // toolbar), so its state lives here with the chart.
  const [groupBy, setGroupBy] = useState<GroupBy>(undefined);
  // Percentile pick as a string so the "All" overlay sentinel shares the state.
  const [percentile, setPercentile] = useState<string>(
    String(DEFAULT_QUANTILE),
  );
  const isAllPercentiles = percentile === PERCENTILE_ALL;
  const [statusCode, setStatusCode] = useState(""); // "" = all
  const discoveredStatusCodes = useMetricsFilterValues(resource, "STATUS_CODE");
  // Host: a dropdown discovered from the App's request-log hosts. Path: free
  // text (not discoverable) committed on Enter/blur, cleared with the × button.
  const [host, setHost] = useState(""); // "" = all
  const discoveredHosts = useLogLabelValues(resource, "host");
  const [pathInput, setPathInput] = useState("");
  const [path, setPath] = useState(""); // committed value driving the query
  const queryOpts = {
    ...window,
    statusCode: statusCode || undefined,
    host: host || undefined,
    path: path || undefined,
  };
  const requests = useMetrics(resource, "http_requests", {
    ...queryOpts,
    groupBy,
  });
  const latency = useMetrics(resource, "http_latency", {
    ...queryOpts,
    ...(isAllPercentiles
      ? { quantiles: ALL_QUANTILES }
      : { quantile: Number(percentile) }),
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
  const latencySeries = useMemo<LineSeriesInput[]>(() => {
    // "All": one line per quantile, named by its `quantile` label (p50/p90/p99).
    if (isAllPercentiles) {
      return latency.series.map((s, i) => ({
        points: s.points,
        color: seriesColor(i),
        label: percentileLabel(s.labels["quantile"]),
      }));
    }
    return [
      { points: latency.series[0]?.points ?? [], color: "var(--chart-5)" },
    ];
  }, [latency.series, isAllPercentiles]);
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
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Select
            value={statusCode === "" ? STATUS_CODE_ALL : statusCode}
            onValueChange={(v) => setStatusCode(v === STATUS_CODE_ALL ? "" : v)}
          >
            <SelectTrigger
              size="sm"
              className="w-36"
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
          <Select
            value={host === "" ? HOST_ALL : host}
            onValueChange={(v) => setHost(v === HOST_ALL ? "" : v)}
          >
            <SelectTrigger
              size="sm"
              className="w-36"
              aria-label={t("metrics.host")}
            >
              <span className="text-muted-foreground">{t("metrics.host")}</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={HOST_ALL}>{t("metrics.hostAll")}</SelectItem>
              {discoveredHosts.map((h) => (
                <SelectItem key={h} value={h}>
                  {h}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="relative">
            <Input
              value={pathInput}
              onChange={(e) => setPathInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") setPath(pathInput.trim());
              }}
              onBlur={() => setPath(pathInput.trim())}
              placeholder={t("metrics.pathPlaceholder")}
              aria-label={t("metrics.path")}
              className="h-8 w-40 pr-7"
            />
            {path !== "" && (
              <button
                type="button"
                onClick={() => {
                  setPath("");
                  setPathInput("");
                }}
                aria-label={t("metrics.pathClear")}
                className="absolute inset-y-0 right-1 flex items-center text-muted-foreground hover:text-foreground"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection
          title={t("metrics.totalRequests")}
          result={requests}
          headerExtra={
            <div className="flex items-center gap-2">
              {requestCount > 0 && (
                <span className="text-xs text-muted-foreground">
                  {t(
                    requestCount === 1
                      ? "metrics.requestCount"
                      : "metrics.requestsCount",
                    { count: requestCount.toLocaleString() },
                  )}
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
            markers={markers}
            markersServiceId={resource}
          />
          <ChartLegend entries={requestSeries} />
        </MetricSection>
        <MetricSection
          title={t("metrics.responseTimes")}
          result={latency}
          headerExtra={
            <Select value={percentile} onValueChange={setPercentile}>
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
                <SelectItem value={PERCENTILE_ALL}>
                  {t("metrics.percentileAll")}
                </SelectItem>
              </SelectContent>
            </Select>
          }
        >
          <SvgLineChart
            unit={latency.series[0]?.unit ?? "seconds"}
            series={latencySeries}
            markers={markers}
            markersServiceId={resource}
          />
          <ChartLegend entries={latencySeries} />
        </MetricSection>
        <MetricSection
          title={t("metrics.outboundBandwidth")}
          result={bandwidth}
          // A real query error is not "No data in range" — the w1/m50 masking
          // fix, now folded into MetricSection's shared error branch (w9/m86)
          // with bandwidth's own message.
          errorMessage={t("metrics.bandwidthError")}
          headerExtra={
            bandwidth.degradedSources.length > 0 ? (
              <BandwidthDegradedBadge sources={bandwidth.degradedSources} />
            ) : undefined
          }
        >
          <SvgLineChart
            unit={bandwidth.series[0]?.unit ?? "bytes"}
            series={bandwidthSeries}
            markers={markers}
            markersServiceId={resource}
          />
        </MetricSection>
        {monthToDate.egressBandwidthMB != null && (
          <p
            className="text-sm text-muted-foreground"
            title={
              monthToDate.degradedSources.length > 0
                ? t("metrics.bandwidthDegradedDetail", {
                    sources: monthToDate.degradedSources.join(", "),
                  })
                : undefined
            }
          >
            {t("metrics.monthToDateBandwidth", {
              amount: formatMegabytes(monthToDate.egressBandwidthMB),
            })}
            {monthToDate.degradedSources.length > 0 ? " *" : null}
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

/** "0.9" → "p90": the overlaid latency line's legend name (percentile "All"). */
function percentileLabel(quantile: string | undefined): string {
  const n = Number(quantile);
  return n > 0 ? `p${Math.round(n * 100)}` : "";
}

/**
 * The bandwidth degradation annotation (w1/m50): an egress source's health
 * product failed inside the window, so the chart still renders but may
 * undercount around the gap. Named sources come from bex-api's
 * degraded_sources label (raw tokens: http/websocket/direct).
 */
function BandwidthDegradedBadge({ sources }: { sources: string[] }) {
  const { t } = useTranslations();
  return (
    <span
      className="text-muted-foreground flex items-center gap-1 text-xs"
      title={t("metrics.bandwidthDegradedDetail", {
        sources: sources.join(", "),
      })}
    >
      <AlertTriangle className="h-3.5 w-3.5 text-amber-500" />
      {t("metrics.bandwidthDegraded")}
    </span>
  );
}

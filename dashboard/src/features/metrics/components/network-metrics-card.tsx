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
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMegabytes } from "@/features/metrics/lib/format";

interface NetworkMetricsCardProps {
  resource: string;
  /** The page's resolved live window (route-owned, shared with the app card). */
  window: UseMetricsOptions;
  quantile: number;
  /** Toolbar Status Code filter ("" = all); see metrics-filters.tsx. */
  statusCode: string;
}

type GroupBy = "status" | "method" | undefined;
const GROUP_BY_ALL = "all";

/**
 * Render's "Network Metrics" card: Total Requests, Response Times, Outbound
 * Bandwidth — Prometheus-backed time-series honoring the range/resolution.
 * The toolbar's Status Code filter applies to requests + latency; bandwidth
 * is deliberately left unfiltered — Traefik's responses-bytes counter carries
 * no `code` label, so filtering it would empty the chart rather than filter
 * it (same honesty rule as the backend's omitted host/path filters).
 */
export function NetworkMetricsCard({
  resource,
  window,
  quantile,
  statusCode,
}: NetworkMetricsCardProps) {
  const { t } = useTranslations();
  // Render puts Group by on the Total Requests chart itself (not the page
  // toolbar), so its state lives here with the chart.
  const [groupBy, setGroupBy] = useState<GroupBy>(undefined);
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

  const requestSeries = useMemo<BarSeriesInput[]>(
    () =>
      requests.series.map((s, i) => ({
        points: s.points,
        color: seriesColor(i, 3), // this chart's base hue is chart-4
        label: groupBy ? groupLabel(s.labels) : undefined,
      })),
    [requests.series, groupBy],
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
      <CardHeader>
        <CardTitle>{t("metrics.networkTitle")}</CardTitle>
        <CardDescription>{t("metrics.networkDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection
          title={t("metrics.totalRequests")}
          result={requests}
          headerExtra={
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
          }
        >
          <SvgBarChart
            unit={requests.series[0]?.unit ?? "count"}
            series={requestSeries}
          />
          <ChartLegend entries={requestSeries} />
        </MetricSection>
        <MetricSection
          title={t("metrics.responseTimes", { quantile })}
          result={latency}
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

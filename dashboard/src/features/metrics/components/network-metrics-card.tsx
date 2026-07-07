import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import { SvgBarChart } from "@/features/metrics/components/svg-bar-chart";
import { SvgLineChart } from "@/features/metrics/components/svg-line-chart";
import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import {
  useMetrics,
  type UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";
import { useMonthToDateBandwidth } from "@/features/metrics/hooks/use-month-to-date-bandwidth";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RangePreset } from "@/features/metrics/lib/range";
import { formatMegabytes } from "@/features/metrics/lib/format";

interface NetworkMetricsCardProps {
  resource: string;
  range: RangePreset;
  quantile: number;
}

/**
 * Render's "Network Metrics" card: Total Requests, Response Times, Outbound
 * Bandwidth — real Prometheus-backed time-series (unlike the resource-metric
 * card, these honor the range/resolution and can have many points).
 */
export function NetworkMetricsCard({
  resource,
  range,
  quantile,
}: NetworkMetricsCardProps) {
  const { t } = useTranslations();
  // useLiveRange already forces a refetch every tick by changing startTime/
  // endTime — Apollo's own pollInterval would just be a second, redundant
  // timer re-issuing the same three queries on its own out-of-phase schedule.
  const queryOpts = { ...useLiveRange(range), pollIntervalMs: 0 };
  const requests = useMetrics(resource, "http_requests", queryOpts);
  const latency = useMetrics(resource, "http_latency", {
    ...queryOpts,
    quantile,
  });
  const bandwidth = useMetrics(resource, "bandwidth", queryOpts);
  const monthToDate = useMonthToDateBandwidth(resource);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("metrics.networkTitle")}</CardTitle>
        <CardDescription>{t("metrics.networkDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection title={t("metrics.totalRequests")} result={requests}>
          {(series) => (
            <SvgBarChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-4)"
            />
          )}
        </MetricSection>
        <MetricSection
          title={t("metrics.responseTimes", { quantile })}
          result={latency}
        >
          {(series) => (
            <SvgLineChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-5)"
            />
          )}
        </MetricSection>
        <MetricSection title={t("metrics.outboundBandwidth")} result={bandwidth}>
          {(series) => (
            <SvgLineChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-1)"
            />
          )}
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

interface MetricSectionProps {
  title: string;
  result: UseMetricsResult;
  children: (series: {
    unit: string;
    points: { timestamp: string; value: number }[];
  }) => React.ReactNode;
}

function MetricSection({ title, result, children }: MetricSectionProps) {
  const series = result.series[0] ?? { unit: "", points: [] };
  return (
    <div>
      <div className="mb-2 text-sm font-medium text-foreground">{title}</div>
      {result.unavailable ? <MetricUnavailable /> : children(series)}
    </div>
  );
}

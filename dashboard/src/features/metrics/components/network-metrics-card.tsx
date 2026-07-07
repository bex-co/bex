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
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import type { RangePreset } from "@/features/metrics/lib/range";

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
  const queryOpts = useLiveRange(range);
  const requests = useMetrics(resource, "http_requests", queryOpts);
  const latency = useMetrics(resource, "http_latency", {
    ...queryOpts,
    quantile,
  });
  const bandwidth = useMetrics(resource, "bandwidth", queryOpts);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Network Metrics</CardTitle>
        <CardDescription>Aggregated across all instances</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection title="Total Requests" result={requests}>
          {(series) => (
            <SvgBarChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-4)"
            />
          )}
        </MetricSection>
        <MetricSection title={`Response Times (${quantile})`} result={latency}>
          {(series) => (
            <SvgLineChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-5)"
            />
          )}
        </MetricSection>
        <MetricSection title="Outbound Bandwidth" result={bandwidth}>
          {(series) => (
            <SvgLineChart
              points={series.points}
              unit={series.unit}
              color="var(--chart-1)"
            />
          )}
        </MetricSection>
      </CardContent>
    </Card>
  );
}

interface MetricSectionProps {
  title: string;
  result: ReturnType<typeof useMetrics>;
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

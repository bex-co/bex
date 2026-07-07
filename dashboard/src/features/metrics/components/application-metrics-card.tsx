import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import { StatTile } from "@/features/metrics/components/stat-tile";
import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import { useMetrics } from "@/features/metrics/hooks/use-metrics";
import { useTranslations } from "@/common/hooks/use-translations";

interface ApplicationMetricsCardProps {
  resource: string;
  percentage: boolean;
}

/**
 * Render's "Application Metrics" card: Memory, CPU, Total Instances. These are
 * metrics-server snapshots (one current point, not a series — docs/
 * observability.md), so each renders as a stat, not a fabricated trend line.
 *
 * PoC scope: takes the first returned series' latest value. bex-api returns
 * one series per replica for cpu/memory; a multi-replica App would need a
 * sum (absolute) or average (percentage) across instances, which this PoC
 * doesn't implement — `beancount-cms`, the milestone's verification target,
 * runs a single replica, so this is not a gap for that target.
 */
export function ApplicationMetricsCard({
  resource,
  percentage,
}: ApplicationMetricsCardProps) {
  const { t } = useTranslations();
  const cpu = useMetrics(resource, "cpu", { percentage });
  const memory = useMetrics(resource, "memory", { percentage });
  const instances = useMetrics(resource, "instance_count");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("metrics.applicationTitle")}</CardTitle>
        <CardDescription>{t("metrics.applicationDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {memory.unavailable ? (
          <MetricUnavailable height={88} />
        ) : (
          <StatTile
            label={t("metrics.memory")}
            unit={memory.series[0]?.unit ?? "bytes"}
            value={latestValue(memory.series)}
            color="var(--chart-1)"
            loading={memory.loading}
          />
        )}
        {cpu.unavailable ? (
          <MetricUnavailable height={88} />
        ) : (
          <StatTile
            label={t("metrics.cpu")}
            unit={cpu.series[0]?.unit ?? "cpu"}
            value={latestValue(cpu.series)}
            color="var(--chart-2)"
            loading={cpu.loading}
          />
        )}
        {instances.unavailable ? (
          <MetricUnavailable height={88} />
        ) : (
          <StatTile
            label={t("metrics.totalInstances")}
            unit="count"
            value={latestValue(instances.series)}
            color="var(--chart-3)"
            loading={instances.loading}
          />
        )}
      </CardContent>
    </Card>
  );
}

function latestValue(series: { points: { value: number }[] }[]): number | null {
  const points = series[0]?.points ?? [];
  return points.length > 0 ? points[points.length - 1].value : null;
}

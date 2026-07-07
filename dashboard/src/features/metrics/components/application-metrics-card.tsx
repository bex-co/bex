import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import { StatTile } from "@/features/metrics/components/stat-tile";
import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import {
  useMetrics,
  type UseMetricsResult,
} from "@/features/metrics/hooks/use-metrics";
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
 * Percentage/Total is computed client-side from two already-fetched series
 * (the raw metric + its _limit counterpart, aggregated to one max value) —
 * captured live from Render's dashboard: it fetches both regardless of which
 * tab is selected rather than asking the backend for a percentage.
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
  const cpu = useMetrics(resource, "cpu");
  const memory = useMetrics(resource, "memory");
  const cpuLimit = useMetrics(resource, "cpu_limit", { aggregateMax: true });
  const memoryLimit = useMetrics(resource, "memory_limit", {
    aggregateMax: true,
  });
  const instances = useMetrics(resource, "instance_count");

  const memoryStat = resourceStat(memory, memoryLimit, percentage, "bytes");
  const cpuStat = resourceStat(cpu, cpuLimit, percentage, "cpu");

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
            unit={memoryStat.unit}
            value={memoryStat.value}
            color="var(--chart-1)"
            loading={memory.loading}
          />
        )}
        {cpu.unavailable ? (
          <MetricUnavailable height={88} />
        ) : (
          <StatTile
            label={t("metrics.cpu")}
            unit={cpuStat.unit}
            value={cpuStat.value}
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

/**
 * Absolute value (raw metric's own unit) or a 0..100 percentage of the
 * matching _limit metric — undefined (null) rather than divide-by-zero when
 * the limit series is empty (no pod limit configured, or not yet loaded).
 */
function resourceStat(
  raw: UseMetricsResult,
  limit: UseMetricsResult,
  percentage: boolean,
  fallbackUnit: string,
): { unit: string; value: number | null } {
  if (!percentage) {
    return { unit: raw.series[0]?.unit ?? fallbackUnit, value: latestValue(raw.series) };
  }
  const rawValue = latestValue(raw.series);
  const limitValue = latestValue(limit.series);
  if (rawValue == null || limitValue == null || limitValue === 0) {
    return { unit: "percentage", value: null };
  }
  return { unit: "percentage", value: (rawValue / limitValue) * 100 };
}

function latestValue(series: { points: { value: number }[] }[]): number | null {
  const points = series[0]?.points ?? [];
  return points.length > 0 ? points[points.length - 1].value : null;
}

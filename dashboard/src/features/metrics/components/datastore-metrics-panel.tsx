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
import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import { useDatastoreMetrics } from "@/features/metrics/hooks/use-datastore-metrics";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMetricValue } from "@/features/metrics/lib/format";
import { latestValue } from "@/features/metrics/lib/series";
import type { ChartSeries, DatastoreKind } from "@/features/metrics/types";

/** Maps a ChartSeries list onto the line chart's per-instance input shape. */
function toLineSeries(series: ChartSeries[]): LineSeriesInput[] {
  return series.map((s, i) => ({
    points: s.points,
    color: seriesColor(i),
    label: s.labels["instance"],
  }));
}

interface DatastoreMetricsPanelProps {
  kind: DatastoreKind;
  /** The Database or KeyValue name. */
  resource: string;
  /**
   * Gates the replication-lag chart (w3/m10, w1/m22): omitted/false for every
   * instance today (Postgres HA hasn't shipped), so the section renders a
   * clear N/A state rather than a broken/empty chart. Ignored for keyvalue.
   */
  highAvailabilityEnabled?: boolean;
}

/**
 * The managed-datastore sibling of ApplicationMetricsCard/NetworkMetricsCard
 * (w3/m10): disk usage for both Postgres and Key Value, plus
 * active-connections and replication-lag for Postgres only. bex extension —
 * Render's dashboard has no equivalent panel.
 */
export function DatastoreMetricsPanel({
  kind,
  resource,
  highAvailabilityEnabled,
}: DatastoreMetricsPanelProps) {
  const { t } = useTranslations();
  const isDatabase = kind === "database";

  const disk = useDatastoreMetrics(kind, resource, "disk");
  const diskCapacity = useDatastoreMetrics(kind, resource, "disk_capacity");
  const connections = useDatastoreMetrics(kind, resource, "db_connections", {
    skip: !isDatabase,
  });
  const replicationLag = useDatastoreMetrics(
    kind,
    resource,
    "replication_lag",
    { skip: !isDatabase || !highAvailabilityEnabled },
  );

  const diskSeries = useMemo(() => toLineSeries(disk.series), [disk.series]);
  const diskCapacityValue = latestValue(diskCapacity.series);

  const connectionSeries = useMemo(
    () => toLineSeries(connections.series),
    [connections.series],
  );

  const replicationLagSeries = useMemo<LineSeriesInput[]>(
    () => [
      {
        points: replicationLag.series[0]?.points ?? [],
        color: "var(--chart-2)",
      },
    ],
    [replicationLag.series],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("metrics.datastoreMetricsTitle")}</CardTitle>
        <CardDescription>
          {t("metrics.datastoreMetricsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection
          title={t("metrics.diskTitle")}
          result={disk}
          headerExtra={
            diskCapacityValue != null ? (
              <span className="text-xs text-muted-foreground">
                {t("metrics.diskCapacityLabel", {
                  value: formatMetricValue("bytes", diskCapacityValue),
                })}
              </span>
            ) : undefined
          }
        >
          <SvgLineChart
            unit={disk.series[0]?.unit ?? "bytes"}
            series={diskSeries}
            referenceValue={diskCapacityValue ?? undefined}
          />
          <ChartLegend entries={diskSeries} />
        </MetricSection>

        {isDatabase && (
          <>
            <MetricSection
              title={t("metrics.connectionsTitle")}
              result={connections}
            >
              <SvgLineChart
                unit={connections.series[0]?.unit ?? "count"}
                series={connectionSeries}
              />
              <ChartLegend entries={connectionSeries} />
            </MetricSection>

            <div>
              <div className="mb-2 text-sm font-medium text-foreground">
                {t("metrics.replicationLagTitle")}
              </div>
              {!highAvailabilityEnabled ? (
                <EmptyChart message={t("metrics.replicationLagPendingHA")} />
              ) : replicationLag.unavailable ? (
                <MetricUnavailable />
              ) : (
                <SvgLineChart
                  unit={replicationLag.series[0]?.unit ?? "seconds"}
                  series={replicationLagSeries}
                />
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

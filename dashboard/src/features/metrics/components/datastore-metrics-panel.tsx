import { useMemo, type ReactNode } from "react";
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
import { MetricError } from "@/features/metrics/components/metric-error";
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
  /** The Database or KeyValue typed id (`dpg-…`/`red-…`) — bex-api resolves it as the CR name; never the display name. */
  resource: string;
  /**
   * Gates the replication-lag chart: omitted for a non-HA instance (no standby
   * means CNPG's lag query returns 0, not absence — the chart renders a clear
   * N/A state rather than a fabricated zero). Ignored for keyvalue.
   */
  highAvailabilityEnabled?: boolean;
  /** Database-only control rendered beside the disk chart heading. */
  diskHeaderExtra?: ReactNode;
  /**
   * Card heading overrides. A service's attached disk (kind="service", ADR082)
   * renders this same panel — the series, the capacity reference line, and the
   * unavailable/error states are identical, because what is being measured is
   * identical: a PVC. Only the words around it differ, so only the words are a
   * prop. Default to the datastore copy.
   */
  title?: string;
  description?: string;
}

/**
 * The managed-datastore sibling of ApplicationMetricsCard/NetworkMetricsCard
 * (w3/m10): disk usage for both Postgres and Key Value, active-connections and
 * replication-lag for Postgres only, and memory + connected-clients for Key
 * Value only (w5/011, redis_exporter). bex extension — Render's dashboard has
 * no equivalent panel.
 *
 * kind="service" (w1/m86) renders just the disk section for the persistent disk
 * attached to a service: a service has no datastore process, so every other
 * chart is already gated off by isDatabase/isKeyValue.
 */
export function DatastoreMetricsPanel({
  kind,
  resource,
  highAvailabilityEnabled,
  diskHeaderExtra,
  title,
  description,
}: DatastoreMetricsPanelProps) {
  const { t } = useTranslations();
  const isDatabase = kind === "database";
  const isKeyValue = kind === "keyvalue";

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
  // Key Value only: used-memory bytes and connected-clients count (w5/011).
  const kvMemory = useDatastoreMetrics(kind, resource, "kv_memory", {
    skip: !isKeyValue,
  });
  const kvConnections = useDatastoreMetrics(kind, resource, "kv_connections", {
    skip: !isKeyValue,
  });

  const diskSeries = useMemo(() => toLineSeries(disk.series), [disk.series]);
  const diskCapacityValue = latestValue(diskCapacity.series);

  const connectionSeries = useMemo(
    () => toLineSeries(connections.series),
    [connections.series],
  );

  const kvMemorySeries = useMemo(
    () => toLineSeries(kvMemory.series),
    [kvMemory.series],
  );
  const kvConnectionSeries = useMemo(
    () => toLineSeries(kvConnections.series),
    [kvConnections.series],
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
        <CardTitle>{title ?? t("metrics.datastoreMetricsTitle")}</CardTitle>
        <CardDescription>
          {description ?? t("metrics.datastoreMetricsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <MetricSection
          title={t("metrics.diskTitle")}
          result={disk}
          headerExtra={
            <div className="flex flex-wrap items-center justify-end gap-3">
              {diskCapacityValue != null ? (
                <span className="text-xs text-muted-foreground">
                  {t("metrics.diskCapacityLabel", {
                    value: formatMetricValue("bytes", diskCapacityValue),
                  })}
                </span>
              ) : null}
              {diskHeaderExtra}
            </div>
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
              ) : replicationLag.error &&
                replicationLag.series.length === 0 ? (
                // This chart renders outside MetricSection, so it needs the same
                // error-vs-empty distinction the shared seam gained (w9/m86).
                <MetricError />
              ) : (
                <SvgLineChart
                  unit={replicationLag.series[0]?.unit ?? "seconds"}
                  series={replicationLagSeries}
                />
              )}
            </div>
          </>
        )}

        {isKeyValue && (
          <>
            <MetricSection title={t("metrics.memoryTitle")} result={kvMemory}>
              <SvgLineChart
                unit={kvMemory.series[0]?.unit ?? "bytes"}
                series={kvMemorySeries}
              />
              <ChartLegend entries={kvMemorySeries} />
            </MetricSection>

            <MetricSection
              title={t("metrics.kvConnectionsTitle")}
              result={kvConnections}
            >
              <SvgLineChart
                unit={kvConnections.series[0]?.unit ?? "count"}
                series={kvConnectionSeries}
              />
              <ChartLegend entries={kvConnectionSeries} />
            </MetricSection>
          </>
        )}
      </CardContent>
    </Card>
  );
}

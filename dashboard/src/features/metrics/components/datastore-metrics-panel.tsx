import { useMemo, useState, type ReactNode } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  SvgLineChart,
  ChartLegend,
  EmptyChart,
  type LineSeriesInput,
} from "@/features/metrics/components/svg-line-chart";
import { SvgBarChart } from "@/features/metrics/components/svg-bar-chart";
import { MetricSection } from "@/features/metrics/components/metric-section";
import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import { MetricError } from "@/features/metrics/components/metric-error";
import { RangeSelect } from "@/features/metrics/components/range-select";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import { useDatastoreMetrics } from "@/features/metrics/hooks/use-datastore-metrics";
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import { useMetricsRetentionBound } from "@/features/metrics/hooks/use-metrics-retention";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import {
  formatDiskCapacity,
  formatMetricValue,
} from "@/features/metrics/lib/format";
import {
  DEFAULT_RANGE_PRESET,
  type RangeSelection,
} from "@/features/metrics/lib/range";
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

/**
 * The two-series graphs (Network Activity, Disk Activity, Disk Operations,
 * w7/m430): each instance's read series before its write series, named by
 * direction, and by instance too once there is more than one.
 */
function toDirectionSeries(
  series: ChartSeries[],
  t: Translate,
): LineSeriesInput[] {
  const instances = new Set(series.map((s) => s.labels["instance"]));
  const rank = (s: ChartSeries) => (s.labels["direction"] === "write" ? 1 : 0);
  return [...series]
    .sort(
      (a, b) =>
        (a.labels["instance"] ?? "").localeCompare(
          b.labels["instance"] ?? "",
        ) || rank(a) - rank(b),
    )
    .map((s, i) => {
      const direction = t(
        rank(s) ? "metrics.directionWrite" : "metrics.directionRead",
      );
      return {
        points: s.points,
        color: seriesColor(i),
        label:
          instances.size > 1
            ? t("metrics.seriesInstanceDirection", {
                instance: s.labels["instance"] ?? "",
                direction,
              })
            : direction,
      };
    });
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
   * Key Value only: the plan's connection limit, the "Limit" Render's Active
   * Connections chart reads beside its title (w6/m459). Omitted until the
   * plan catalog says it.
   */
  connectionLimit?: number;
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
 * (w3/m10): disk usage for both Postgres and Key Value, CPU and memory against
 * their limits for both (w1/m180; Key Value memory is its used memory, w5/011),
 * active-connections and replication-lag for Postgres only, and
 * connected-clients for Key Value only, under the title Render gives both,
 * with the plan's connection limit beside it for Key Value (w6/m459).
 *
 * Render's database-activity and disk graphs (w7/m430) sit where Render puts
 * them: Disk Activity under disk usage for all three kinds, Disk Operations for
 * Postgres and a service's disk, and Network Activity, Transaction Volume and
 * Lock-Delayed Queries among Postgres's database activity.
 *
 * kind="service" (w1/m86) renders just the disk sections for the persistent
 * disk attached to a service: a service has no datastore process, so every
 * other chart is already gated off by isDatabase/isKeyValue.
 *
 * Time is the panel's one control, as it is the service Metrics page's
 * (w7/072): Render's preset ladder from the 12-hour default, bounded by how
 * far back the workspace plan keeps metrics (w7/m427), and every read covers
 * the chosen range.
 */
export function DatastoreMetricsPanel({
  kind,
  resource,
  highAvailabilityEnabled,
  diskHeaderExtra,
  connectionLimit,
  title,
  description,
}: DatastoreMetricsPanelProps) {
  const { t, i18n } = useTranslations();
  const isDatabase = kind === "database";
  const isKeyValue = kind === "keyvalue";
  // A service's disk has no datastore process, so no CPU or memory.
  const isDatastore = isDatabase || isKeyValue;

  const [range, setRange] = useState<RangeSelection>(DEFAULT_RANGE_PRESET);
  const rangeBound = useMetricsRetentionBound(resource);
  // ONE live window for every read, as on the service Metrics page: its tick
  // refetches by moving start/end, so Apollo's own poll timer stays off. The
  // limit and capacity reads take it too, so a past custom range labels the
  // limit that applied then.
  const live = { ...useLiveRange(range), pollIntervalMs: 0 };

  const disk = useDatastoreMetrics(kind, resource, "disk", live);
  const diskCapacity = useDatastoreMetrics(
    kind,
    resource,
    "disk_capacity",
    live,
  );
  const connections = useDatastoreMetrics(kind, resource, "db_connections", {
    ...live,
    skip: !isDatabase,
  });
  const replicationLag = useDatastoreMetrics(
    kind,
    resource,
    "replication_lag",
    { ...live, skip: !isDatabase || !highAvailabilityEnabled },
  );
  // Key Value only: connected-clients count (w5/011).
  const kvConnections = useDatastoreMetrics(kind, resource, "kv_connections", {
    ...live,
    skip: !isKeyValue,
  });
  // CPU and memory (w1/m180): Postgres's own container; a Key Value's memory
  // is its used memory (w5/011). Its container limit is no line for that
  // chart: under a maxmemory policy Valkey evicts at 80% of it.
  const cpu = useDatastoreMetrics(kind, resource, "cpu", {
    ...live,
    skip: !isDatastore,
  });
  const cpuLimit = useDatastoreMetrics(kind, resource, "cpu_limit", {
    ...live,
    skip: !isDatastore,
  });
  const memory = useDatastoreMetrics(kind, resource, "memory", {
    ...live,
    skip: !isDatastore,
  });
  const memoryLimit = useDatastoreMetrics(kind, resource, "memory_limit", {
    ...live,
    skip: !isDatabase,
  });
  // Render's disk and database-activity graphs (w7/m430). A Key Value has
  // Disk Activity only; the database-activity graphs are Postgres's.
  const diskActivity = useDatastoreMetrics(
    kind,
    resource,
    "disk_activity",
    live,
  );
  const diskOperations = useDatastoreMetrics(
    kind,
    resource,
    "disk_operations",
    { ...live, skip: isKeyValue },
  );
  const networkActivity = useDatastoreMetrics(
    kind,
    resource,
    "network_activity",
    { ...live, skip: !isDatabase },
  );
  const transactionVolume = useDatastoreMetrics(
    kind,
    resource,
    "transaction_volume",
    { ...live, skip: !isDatabase },
  );
  const lockDelayedQueries = useDatastoreMetrics(
    kind,
    resource,
    "lock_delayed_queries",
    { ...live, skip: !isDatabase },
  );

  const diskSeries = useMemo(() => toLineSeries(disk.series), [disk.series]);
  const diskCapacityValue = latestValue(diskCapacity.series);

  const connectionSeries = useMemo(
    () => toLineSeries(connections.series),
    [connections.series],
  );

  const cpuSeries = useMemo(() => toLineSeries(cpu.series), [cpu.series]);
  const cpuLimitValue = latestValue(cpuLimit.series);
  const memorySeries = useMemo(
    () => toLineSeries(memory.series),
    [memory.series],
  );
  const memoryLimitValue = latestValue(memoryLimit.series);
  const limitText = (value: string | null) =>
    value != null ? (
      <span className="text-xs text-muted-foreground">
        {t("metrics.datastoreLimitLabel", { value })}
      </span>
    ) : null;
  const limitLabel = (unit: string, value: number | null) =>
    limitText(value != null ? formatMetricValue(unit, value, t) : null);

  const kvConnectionSeries = useMemo(
    () => toLineSeries(kvConnections.series),
    [kvConnections.series],
  );

  const diskActivitySeries = useMemo(
    () => toDirectionSeries(diskActivity.series, t),
    [diskActivity.series, t],
  );
  const diskOperationSeries = useMemo(
    () => toDirectionSeries(diskOperations.series, t),
    [diskOperations.series, t],
  );
  const networkSeries = useMemo(
    () => toDirectionSeries(networkActivity.series, t),
    [networkActivity.series, t],
  );
  const transactionSeries = useMemo(
    () => toLineSeries(transactionVolume.series),
    [transactionVolume.series],
  );
  const lockDelayedSeries = useMemo(
    () => toLineSeries(lockDelayedQueries.series),
    [lockDelayedQueries.series],
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
        {/* A wrapping row, not the header's action slot: an applied custom
            range widens the trigger and adds its Edit button, which the
            header's fixed column cannot hold at phone width. */}
        <div className="flex flex-wrap items-center gap-3">
          <RangeSelect
            range={range}
            onRangeChange={setRange}
            bound={rangeBound}
          />
        </div>

        <MetricSection
          title={t("metrics.diskTitle")}
          result={disk}
          headerExtra={
            <div className="flex flex-wrap items-center justify-end gap-3">
              {diskCapacityValue != null ? (
                <span className="text-xs text-muted-foreground">
                  {t("metrics.diskCapacityLabel", {
                    value: formatDiskCapacity(diskCapacityValue),
                  })}
                </span>
              ) : diskCapacity.loading ? (
                // Hold the label's slot while it loads: beside the Postgres
                // autoscaling box at phone width it takes a row of its own,
                // so arriving late would push the chart down.
                <Skeleton
                  aria-hidden="true"
                  data-testid="disk-capacity-pending"
                  className="h-4 w-24"
                />
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

        <MetricSection
          title={t("metrics.diskActivityTitle")}
          result={diskActivity}
        >
          <SvgLineChart
            unit={diskActivity.series[0]?.unit ?? "bytes"}
            series={diskActivitySeries}
          />
          <ChartLegend entries={diskActivitySeries} />
        </MetricSection>

        {!isKeyValue && (
          <MetricSection
            title={t("metrics.diskOperationsTitle")}
            result={diskOperations}
          >
            <SvgLineChart
              unit={diskOperations.series[0]?.unit ?? "count"}
              series={diskOperationSeries}
            />
            <ChartLegend entries={diskOperationSeries} />
          </MetricSection>
        )}

        {isDatastore && (
          <>
            <MetricSection
              title={t("metrics.datastoreCpuTitle")}
              result={cpu}
              headerExtra={limitLabel("cpu", cpuLimitValue)}
            >
              <SvgLineChart
                unit={cpu.series[0]?.unit ?? "cpu"}
                series={cpuSeries}
                referenceValue={cpuLimitValue ?? undefined}
              />
              <ChartLegend entries={cpuSeries} />
            </MetricSection>

            <MetricSection
              title={t("metrics.memoryTitle")}
              result={memory}
              headerExtra={limitLabel("bytes", memoryLimitValue)}
            >
              <SvgLineChart
                unit={memory.series[0]?.unit ?? "bytes"}
                series={memorySeries}
                referenceValue={memoryLimitValue ?? undefined}
              />
              <ChartLegend entries={memorySeries} />
            </MetricSection>
          </>
        )}

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

            <MetricSection
              title={t("metrics.networkActivityTitle")}
              result={networkActivity}
            >
              <SvgLineChart
                unit={networkActivity.series[0]?.unit ?? "bytes"}
                series={networkSeries}
              />
              <ChartLegend entries={networkSeries} />
            </MetricSection>

            <MetricSection
              title={t("metrics.transactionVolumeTitle")}
              result={transactionVolume}
            >
              <SvgLineChart
                unit={transactionVolume.series[0]?.unit ?? "count"}
                series={transactionSeries}
              />
              <ChartLegend entries={transactionSeries} />
            </MetricSection>

            <div>
              <div className="mb-2 text-sm font-medium text-foreground">
                {t("metrics.replicationLagTitle")}
              </div>
              {!highAvailabilityEnabled ? (
                <EmptyChart message={t("metrics.replicationLagPendingHA")} />
              ) : replicationLag.unavailable ? (
                <MetricUnavailable />
              ) : replicationLag.error && replicationLag.series.length === 0 ? (
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

            {/* Counted when a wait ends, so its bars are events, not a level. */}
            <MetricSection
              title={t("metrics.lockDelayedQueriesTitle")}
              result={lockDelayedQueries}
              storeUnavailableMessage={t("metrics.lockDelayedStoreUnavailable")}
            >
              <SvgBarChart
                unit={lockDelayedQueries.series[0]?.unit ?? "count"}
                series={lockDelayedSeries}
              />
              <ChartLegend entries={lockDelayedSeries} />
            </MetricSection>
          </>
        )}

        {isKeyValue && (
          <MetricSection
            title={t("metrics.connectionsTitle")}
            result={kvConnections}
            // The whole number, as Render's "Limit 1,000"; the chart's own
            // count format would round it to "1.0K".
            headerExtra={limitText(
              connectionLimit
                ? new Intl.NumberFormat(i18n.language).format(connectionLimit)
                : null,
            )}
          >
            <SvgLineChart
              unit={kvConnections.series[0]?.unit ?? "count"}
              series={kvConnectionSeries}
            />
            <ChartLegend entries={kvConnectionSeries} />
          </MetricSection>
        )}
      </CardContent>
    </Card>
  );
}

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { StyleSheet, Text, View } from "react-native";
import { DashboardCard } from "@/components/dashboard-card";
import { useFreshness } from "@/common/hooks/freshness";
import { useRecoveryEnvironment } from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { useTranslations } from "@/common/hooks/use-translations";
import { fonts, fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import {
  MobilePostgresCapacityDocument,
  MobilePostgresProcessesDocument,
  MobilePostgresSizesDocument,
  MobilePostgresTableScansDocument,
} from "@/generated-graphql";
import {
  adaptMetricSeries,
  formatMetricValue,
} from "@/features/metrics/series";
import {
  compactPostgresTableInsights,
  POSTGRES_INSIGHT_STALE_AFTER_MS,
  postgresInsightState,
  summarizePostgresProcesses,
  type PostgresInsightState,
} from "./insights";

const POLL_INTERVAL_MS = 30_000;

export type PostgresInsightsCardHandle = { refresh: () => Promise<void> };

export const PostgresInsightsCard = forwardRef<
  PostgresInsightsCardHandle,
  { databaseId: string }
>(function PostgresInsightsCard({ databaseId }, ref) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const recoveryEnvironment = useRecoveryEnvironment();
  const pollInterval = recoveryAvailable(recoveryEnvironment)
    ? POLL_INTERVAL_MS
    : 0;
  const queryOptions = {
    variables: { id: databaseId },
    fetchPolicy: "cache-and-network" as const,
    errorPolicy: "all" as const,
    notifyOnNetworkStatusChange: true,
    pollInterval,
  };
  const capacity = useQuery(MobilePostgresCapacityDocument, queryOptions);
  const sizes = useQuery(MobilePostgresSizesDocument, queryOptions);
  const processes = useQuery(MobilePostgresProcessesDocument, queryOptions);
  const scans = useQuery(MobilePostgresTableScansDocument, queryOptions);
  useImperativeHandle(
    ref,
    () => ({
      refresh: async () => {
        await Promise.all([
          capacity.refetch({ id: databaseId }),
          sizes.refetch({ id: databaseId }),
          processes.refetch({ id: databaseId }),
          scans.refetch({ id: databaseId }),
        ]);
      },
    }),
    [capacity, databaseId, processes, scans, sizes],
  );

  const capacityHasData = Boolean(
    capacity.data?.disk ||
    capacity.data?.capacity ||
    capacity.data?.connections,
  );
  const sizesState = usePostgresInsightState(
    sizes.data?.databaseSizes != null,
    Boolean(sizes.error),
    sizes.networkStatus,
  );
  const processesState = usePostgresInsightState(
    processes.data?.databaseProcesses != null,
    Boolean(processes.error),
    processes.networkStatus,
  );
  const scansState = usePostgresInsightState(
    scans.data?.databaseTableScans != null,
    Boolean(scans.error),
    scans.networkStatus,
  );

  const disk = useMemo(
    () => adaptMetricSeries(capacity.data?.disk),
    [capacity.data?.disk],
  );
  const diskCapacity = useMemo(
    () => adaptMetricSeries(capacity.data?.capacity),
    [capacity.data?.capacity],
  );
  const connections = useMemo(
    () => adaptMetricSeries(capacity.data?.connections),
    [capacity.data?.connections],
  );
  const capacityState = usePostgresInsightState(
    capacityHasData,
    Boolean(capacity.error),
    capacity.networkStatus,
    newestMetricTimestamp([disk, diskCapacity, connections]),
  );
  const processSummary = useMemo(
    () => summarizePostgresProcesses(processes.data?.databaseProcesses),
    [processes.data?.databaseProcesses],
  );
  const tables = useMemo(
    () =>
      compactPostgresTableInsights(
        sizes.data?.databaseSizes?.tables,
        scans.data?.databaseTableScans,
      ),
    [sizes.data?.databaseSizes?.tables, scans.data?.databaseTableScans],
  );
  const connectionState = mergeInsightState(sizesState, processesState);
  const capacitySectionState = mergeInsightState(capacityState, sizesState);

  return (
    <DashboardCard title={t("postgresInsights.title")}>
      <InsightSection
        title={t("postgresInsights.connection")}
        state={connectionState}
      >
        <Text style={[styles.primaryValue, { color: theme.foreground }]}>
          {t(`postgresInsights.connectionState.${connectionState}`)}
        </Text>
      </InsightSection>

      <InsightSection
        title={t("postgresInsights.capacity")}
        state={capacitySectionState}
      >
        <InsightRow
          label={t("postgresInsights.logicalSize")}
          value={sizes.data?.databaseSizes?.database?.sizePretty ?? "—"}
        />
        <InsightRow
          label={t("postgresInsights.diskUsed")}
          value={formatMetric(disk.unit, disk.current)}
        />
        <InsightRow
          label={t("postgresInsights.diskCapacity")}
          value={formatMetric(diskCapacity.unit, diskCapacity.current)}
        />
        <InsightRow
          label={t("postgresInsights.connections")}
          value={formatMetric(connections.unit, connections.current)}
        />
      </InsightSection>

      <InsightSection
        title={t("postgresInsights.processes")}
        state={processesState}
      >
        {processesState === "unavailable" ? null : (
          <>
            <InsightRow
              label={t("postgresInsights.activeProcesses")}
              value={`${processSummary.active} / ${processSummary.total}`}
            />
            <InsightRow
              label={t("postgresInsights.waitingProcesses")}
              value={processSummary.waiting.toLocaleString()}
            />
            <InsightRow
              label={t("postgresInsights.longestProcess")}
              value={
                processSummary.longestSeconds == null
                  ? "—"
                  : t("postgresInsights.seconds", {
                      value: processSummary.longestSeconds,
                    })
              }
            />
          </>
        )}
      </InsightSection>

      <InsightSection
        title={t("postgresInsights.tables")}
        state={mergeInsightState(sizesState, scansState)}
      >
        {tables.length ? (
          tables.map((table) => (
            <View
              key={table.key}
              style={[styles.tableRow, { borderTopColor: theme.border }]}
            >
              <Text
                numberOfLines={1}
                style={[styles.tableName, { color: theme.foreground }]}
              >
                {table.label}
              </Text>
              <Text
                style={[styles.tableMeta, { color: theme.mutedForeground }]}
              >
                {t("postgresInsights.tableSummary", {
                  size: table.sizePretty ?? "—",
                  sequential: formatCounter(table.seqScans),
                  index: formatCounter(table.indexScans),
                  dead: formatCounter(table.deadRows),
                })}
              </Text>
            </View>
          ))
        ) : mergeInsightState(sizesState, scansState) ===
          "unavailable" ? null : (
          <Text style={[styles.note, { color: theme.mutedForeground }]}>
            {t("postgresInsights.noTableData")}
          </Text>
        )}
      </InsightSection>
    </DashboardCard>
  );
});

function InsightSection({
  title,
  state,
  children,
}: {
  title: string;
  state: PostgresInsightState;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const stateColor =
    state === "unavailable"
      ? theme.error
      : state === "degraded" || state === "stale"
        ? theme.warning
        : theme.mutedForeground;
  return (
    <View style={[styles.section, { borderTopColor: theme.border }]}>
      <View style={styles.sectionHeader}>
        <Text style={[styles.sectionTitle, { color: theme.foreground }]}>
          {title}
        </Text>
        <Text style={[styles.state, { color: stateColor }]}>
          {t(`postgresInsights.state.${state}`)}
        </Text>
      </View>
      {state === "unavailable" ? (
        <Text
          accessibilityRole="alert"
          style={[styles.note, { color: stateColor }]}
        >
          {t("postgresInsights.unavailable")}
        </Text>
      ) : (
        children
      )}
    </View>
  );
}

function InsightRow({ label, value }: { label: string; value: string }) {
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.metricRow}>
      <Text style={[styles.metricLabel, { color: theme.mutedForeground }]}>
        {label}
      </Text>
      <Text style={[styles.metricValue, { color: theme.foreground }]}>
        {value}
      </Text>
    </View>
  );
}

function usePostgresInsightState(
  hasData: boolean,
  hasError: boolean,
  networkStatus: NetworkStatus,
  sourceObservedAt?: number | null,
): PostgresInsightState {
  const [receivedAt, setReceivedAt] = useState<number | null>(null);
  useEffect(() => {
    if (
      sourceObservedAt === undefined &&
      hasData &&
      networkStatus === NetworkStatus.ready &&
      !hasError
    ) {
      setReceivedAt(Date.now());
    }
  }, [hasData, hasError, networkStatus, sourceObservedAt]);
  const observedAt =
    sourceObservedAt === undefined ? receivedAt : sourceObservedAt;
  const freshness = useFreshness(observedAt, {
    staleAfterMs: POSTGRES_INSIGHT_STALE_AFTER_MS,
  });
  if (
    sourceObservedAt === null &&
    hasData &&
    !hasError &&
    networkStatus === NetworkStatus.ready
  ) {
    return "empty";
  }
  return postgresInsightState({
    hasData,
    hasError,
    observedAt,
    now: observedAt == null ? undefined : observedAt + (freshness.ageMs ?? 0),
  });
}

function mergeInsightState(
  left: PostgresInsightState,
  right: PostgresInsightState,
): PostgresInsightState {
  if (left === "unavailable" && right === "unavailable") return "unavailable";
  if (left === "unavailable" || right === "unavailable") return "degraded";
  if (left === "degraded" || right === "degraded") return "degraded";
  if (left === "empty" || right === "empty") {
    return left === "current" || right === "current" ? "degraded" : "empty";
  }
  if (left === "stale" || right === "stale") return "stale";
  if (left === "current" || right === "current") return "current";
  return "loading";
}

function newestMetricTimestamp(
  snapshots: Array<{ points: Array<{ timestamp: string }> }>,
): number | null {
  const timestamps = snapshots
    .flatMap((snapshot) => snapshot.points)
    .map((point) => Date.parse(point.timestamp))
    .filter(Number.isFinite);
  return timestamps.length ? Math.max(...timestamps) : null;
}

function formatMetric(unit: string, value: number | null): string {
  return value == null ? "—" : formatMetricValue(unit, value);
}

function formatCounter(value: number | null): string {
  return value == null ? "—" : value.toLocaleString();
}

const styles = StyleSheet.create({
  section: {
    gap: space.sm,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.md,
  },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
  },
  sectionTitle: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  state: { fontSize: fontSizes.xs },
  primaryValue: { fontSize: fontSizes.lg, fontWeight: "600" },
  metricRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    gap: space.md,
  },
  metricLabel: { flex: 1, fontSize: fontSizes.sm },
  metricValue: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
  tableRow: {
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingTop: space.sm,
    gap: space.xs,
  },
  tableName: { fontFamily: fonts.mono, fontSize: fontSizes.sm },
  tableMeta: { fontSize: fontSizes.xs },
  note: { fontSize: fontSizes.sm },
});

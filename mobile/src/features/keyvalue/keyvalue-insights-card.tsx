import { forwardRef, useImperativeHandle, useMemo } from "react";
import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { CompactSparkline } from "@/common/d3/compact-sparkline";
import { useFreshness } from "@/common/hooks/freshness";
import { useRecoveryEnvironment } from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { DashboardCard } from "@/components/dashboard-card";
import {
  formatMetricValue,
  type MetricSnapshot,
} from "@/features/metrics/series";
import { MobileKeyValueInsightsDocument } from "@/generated-graphql";
import {
  buildKeyValueInsightSnapshot,
  keyValueConnectionHealth,
  keyValueMetricFailure,
} from "./insights";

const STALE_AFTER_MS = 65_000;

export type KeyValueInsightsCardHandle = { refresh: () => Promise<void> };

export const KeyValueInsightsCard = forwardRef<
  KeyValueInsightsCardHandle,
  {
    resourceId: string;
    status: string | null | undefined;
    suspended: boolean | string | null | undefined;
  }
>(function KeyValueInsightsCard({ resourceId, status, suspended }, ref) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const recoveryEnvironment = useRecoveryEnvironment();
  const query = useQuery(MobileKeyValueInsightsDocument, {
    variables: { id: resourceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval: recoveryAvailable(recoveryEnvironment) ? 30_000 : 0,
  });
  useImperativeHandle(
    ref,
    () => ({
      refresh: async () => {
        await query.refetch({ id: resourceId });
      },
    }),
    [query, resourceId],
  );

  const snapshot = useMemo(
    () => buildKeyValueInsightSnapshot(query.data),
    [query.data],
  );
  const freshness = useFreshness(
    snapshot.latestAt ? Date.parse(snapshot.latestAt) : null,
    { staleAfterMs: STALE_AFTER_MS },
  );
  const health = keyValueConnectionHealth({ status, suspended });
  const healthColor =
    health === "available"
      ? theme.success
      : health === "unavailable"
        ? theme.error
        : health === "creating"
          ? theme.warning
          : theme.mutedForeground;
  const refreshing = query.networkStatus === NetworkStatus.refetch;
  const data = query.data;
  const diskFailure = keyValueMetricFailure(
    query.error,
    "disk",
    data?.disk != null,
  );
  const capacityFailure = keyValueMetricFailure(
    query.error,
    "diskCapacity",
    data?.diskCapacity != null,
  );
  const storageFailure =
    diskFailure === "unavailable" || capacityFailure === "unavailable"
      ? "unavailable"
      : diskFailure || capacityFailure;

  return (
    <DashboardCard
      title={t("keyValueInsights.title")}
      right={
        refreshing ? (
          <ActivityIndicator
            accessibilityLabel={t("keyValueInsights.refreshing")}
            color={theme.primary}
          />
        ) : undefined
      }
    >
      <Text style={[styles.window, { color: theme.mutedForeground }]}>
        {t("keyValueInsights.window")} · {t(freshness.label)}
      </Text>
      <View style={[styles.health, { borderTopColor: theme.border }]}>
        <Text style={[styles.name, { color: theme.mutedForeground }]}>
          {t("keyValueInsights.connectionHealth")}
        </Text>
        <View style={styles.healthValue}>
          <View style={[styles.dot, { backgroundColor: healthColor }]} />
          <Text style={[styles.value, { color: healthColor }]}>
            {t(`keyValueInsights.health.${health}`)}
          </Text>
        </View>
      </View>
      <InsightRow
        title={t("keyValueInsights.storage")}
        snapshot={snapshot.disk}
        failure={storageFailure}
        value={storageValue(snapshot)}
        freshnessAt={newestTimestamp([snapshot.disk, snapshot.diskCapacity])}
        partial={snapshot.disk.partial || snapshot.diskCapacity.partial}
        loading={query.loading && !data}
      />
      <InsightRow
        title={t("keyValueInsights.memory")}
        snapshot={snapshot.memory}
        failure={keyValueMetricFailure(
          query.error,
          "memory",
          data?.memory != null,
        )}
        value={metricValue(snapshot.memory)}
        freshnessAt={newestTimestamp([snapshot.memory])}
        loading={query.loading && !data}
      />
      <InsightRow
        title={t("keyValueInsights.connections")}
        snapshot={snapshot.connections}
        failure={keyValueMetricFailure(
          query.error,
          "connections",
          data?.connections != null,
        )}
        value={metricValue(snapshot.connections)}
        freshnessAt={newestTimestamp([snapshot.connections])}
        loading={query.loading && !data}
      />
    </DashboardCard>
  );
});

function InsightRow({
  title,
  snapshot,
  failure,
  value,
  freshnessAt,
  partial = snapshot.partial,
  loading,
}: {
  title: string;
  snapshot: ReturnType<typeof buildKeyValueInsightSnapshot>["disk"];
  failure: ReturnType<typeof keyValueMetricFailure>;
  value: string;
  freshnessAt: string | null;
  partial?: boolean;
  loading: boolean;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const freshness = useFreshness(freshnessAt ? Date.parse(freshnessAt) : null, {
    staleAfterMs: STALE_AFTER_MS,
  });
  const note = loading
    ? t("metrics.loading", { metric: title })
    : failure
      ? t(failure === "unavailable" ? "metrics.unavailable" : "metrics.error")
      : partial
        ? t("metrics.partial")
        : snapshot.degradedSources.length
          ? t("metrics.degraded", {
              sources: snapshot.degradedSources.join(", "),
            })
          : freshness.status === "stale" && snapshot.current != null
            ? t("freshness.stale")
            : snapshot.current == null
              ? t("metrics.noData")
              : null;
  return (
    <View style={[styles.row, { borderTopColor: theme.border }]}>
      <View style={styles.copy}>
        <Text style={[styles.name, { color: theme.mutedForeground }]}>
          {title}
        </Text>
        <Text style={[styles.value, { color: theme.foreground }]}>{value}</Text>
        {note ? (
          <Text
            style={[
              styles.note,
              {
                color:
                  failure === "error"
                    ? theme.error
                    : partial ||
                        snapshot.degradedSources.length ||
                        freshness.status === "stale"
                      ? theme.warning
                      : theme.mutedForeground,
              },
            ]}
          >
            {note}
          </Text>
        ) : null}
      </View>
      {snapshot.points.length ? (
        <CompactSparkline
          points={snapshot.points}
          accessibilityLabel={t("keyValueInsights.accessibilitySummary", {
            metric: title,
            value,
            points: snapshot.points.length,
          })}
        />
      ) : null}
    </View>
  );
}

function metricValue(snapshot: {
  unit: string;
  current: number | null;
}): string {
  return snapshot.current == null
    ? "—"
    : formatMetricValue(snapshot.unit, snapshot.current);
}

function storageValue(
  snapshot: ReturnType<typeof buildKeyValueInsightSnapshot>,
): string {
  const used = metricValue(snapshot.disk);
  const capacity = metricValue(snapshot.diskCapacity);
  if (snapshot.disk.current == null && snapshot.diskCapacity.current == null) {
    return "—";
  }
  if (snapshot.diskCapacity.current == null) return used;
  const percent =
    snapshot.diskUsedPercent == null
      ? ""
      : ` (${snapshot.diskUsedPercent.toFixed(1)}%)`;
  return `${used} / ${capacity}${percent}`;
}

function newestTimestamp(snapshots: readonly MetricSnapshot[]): string | null {
  return (
    snapshots
      .flatMap((snapshot) => snapshot.points.map((point) => point.timestamp))
      .filter((timestamp) => Number.isFinite(Date.parse(timestamp)))
      .sort((left, right) => Date.parse(left) - Date.parse(right))
      .at(-1) ?? null
  );
}

const styles = StyleSheet.create({
  window: { fontSize: fontSizes.sm, marginBottom: space.sm },
  health: {
    minHeight: 70,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.md,
  },
  healthValue: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    marginTop: space.xs,
  },
  dot: { width: 10, height: 10, borderRadius: 5 },
  row: {
    minHeight: 94,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.md,
  },
  copy: { flex: 1, minWidth: 110 },
  name: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
  value: { fontSize: fontSizes.lg, fontWeight: "600", marginTop: space.xs },
  note: { fontSize: fontSizes.xs, marginTop: space.xs },
});

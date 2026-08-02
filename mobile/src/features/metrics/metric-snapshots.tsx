import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { DashboardCard } from "@/components/dashboard-card";
import { CompactSparkline } from "@/common/d3/compact-sparkline";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRecoveryEnvironment } from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { MobileMetricSnapshotDocument } from "@/generated-graphql";
import { adaptMetricSeries, formatMetricValue } from "./series";

type MetricName = "CPU" | "MEMORY" | "BANDWIDTH";

export function MetricSnapshots({ resourceId }: { resourceId: string }) {
  const { t } = useTranslations();
  return (
    <DashboardCard title={t("metrics.title")}>
      <Text
        style={[
          styles.window,
          { color: useTheme().colorTheme.mutedForeground },
        ]}
      >
        {t("metrics.window")}
      </Text>
      <MetricSnapshotRow
        resourceId={resourceId}
        metric="CPU"
        title={t("metrics.cpu")}
      />
      <MetricSnapshotRow
        resourceId={resourceId}
        metric="MEMORY"
        title={t("metrics.memory")}
      />
      <MetricSnapshotRow
        resourceId={resourceId}
        metric="BANDWIDTH"
        title={t("metrics.network")}
      />
    </DashboardCard>
  );
}

function MetricSnapshotRow({
  resourceId,
  metric,
  title,
}: {
  resourceId: string;
  metric: MetricName;
  title: string;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const recoveryEnvironment = useRecoveryEnvironment();
  const { data, loading, error } = useQuery(MobileMetricSnapshotDocument, {
    variables: {
      resourceId,
      name: metric,
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: recoveryAvailable(recoveryEnvironment) ? 30_000 : 0,
  });
  const snapshot = useMemo(() => adaptMetricSeries(data?.metrics), [data]);
  const value =
    snapshot.current == null
      ? "—"
      : formatMetricValue(snapshot.unit, snapshot.current);
  const unavailable = error?.message.includes("metrics source not configured");
  const accessibility = t("metrics.accessibilitySummary", {
    metric: title,
    value,
    points: snapshot.points.length,
  });

  return (
    <View style={[styles.row, { borderTopColor: theme.border }]}>
      <View style={styles.copy}>
        <Text style={[styles.name, { color: theme.mutedForeground }]}>
          {title}
        </Text>
        {loading && !data ? (
          <ActivityIndicator
            accessibilityLabel={t("metrics.loading", { metric: title })}
            color={theme.primary}
            style={styles.loader}
          />
        ) : (
          <Text style={[styles.value, { color: theme.foreground }]}>
            {value}
          </Text>
        )}
        {unavailable ? (
          <Text style={[styles.note, { color: theme.mutedForeground }]}>
            {t("metrics.unavailable")}
          </Text>
        ) : error && !data ? (
          <Text style={[styles.note, { color: theme.error }]}>
            {t("metrics.error")}
          </Text>
        ) : snapshot.partial ? (
          <Text style={[styles.note, { color: theme.warning }]}>
            {t("metrics.partial")}
          </Text>
        ) : snapshot.degradedSources.length ? (
          <Text style={[styles.note, { color: theme.warning }]}>
            {t("metrics.degraded", {
              sources: snapshot.degradedSources.join(", "),
            })}
          </Text>
        ) : snapshot.current == null && !loading ? (
          <Text style={[styles.note, { color: theme.mutedForeground }]}>
            {t("metrics.noData")}
          </Text>
        ) : null}
      </View>
      {snapshot.points.length ? (
        <CompactSparkline
          points={snapshot.points}
          accessibilityLabel={accessibility}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  window: { fontSize: fontSizes.sm, marginBottom: space.sm },
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
  value: { fontSize: fontSizes.xl, fontWeight: "600", marginTop: space.xs },
  note: { fontSize: fontSizes.xs, marginTop: space.xs },
  loader: { alignSelf: "flex-start", marginTop: space.md },
});

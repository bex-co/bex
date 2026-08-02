import { StyleSheet, Text, View } from "react-native";
import { DashboardCard } from "@/components/dashboard-card";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import {
  buildUsageGlance,
  formatUsageTotal,
  type UsageCoverageState,
  type UsageSummaryWire,
} from "./usage-glance";

export type UsageGlanceCopy = {
  title: string;
  states: Record<UsageCoverageState, string>;
  meterLabels: Record<string, string>;
  empty: string;
  noEvidence: string;
  through: (timestamp: string) => string;
  degraded: (sources: string) => string;
  refreshUnavailable: string;
};

/**
 * Presentational read-only card. The owner screen owns the Apollo query and
 * passes its successful (possibly cached) payload so refresh failures cannot
 * erase previously observed totals. Copy is injected by that screen to keep
 * this feature compatible with the shared translation catalog.
 */
export function UsageGlanceCard({
  summary,
  unavailable = false,
  copy,
}: {
  summary: UsageSummaryWire | null | undefined;
  unavailable?: boolean;
  copy: UsageGlanceCopy;
}) {
  const theme = useTheme().colorTheme;
  const glance = buildUsageGlance({ summary, unavailable });
  const stateColor =
    glance.state === "unavailable"
      ? theme.error
      : glance.state === "partial" ||
          glance.state === "unknown" ||
          glance.refreshUnavailable
        ? theme.warning
        : theme.mutedForeground;

  return (
    <DashboardCard title={copy.title}>
      <View style={styles.stateRow}>
        <Text style={[styles.period, { color: theme.foreground }]}>
          {glance.period ?? "—"}
        </Text>
        <Text style={[styles.state, { color: stateColor }]}>
          {copy.states[glance.state]}
        </Text>
      </View>

      {glance.totals.length ? (
        <View>
          {glance.totals.map((row) => (
            <View
              key={row.kind}
              style={[styles.totalRow, { borderTopColor: theme.border }]}
            >
              <Text style={[styles.label, { color: theme.mutedForeground }]}>
                {copy.meterLabels[row.kind] ?? row.kind}
              </Text>
              <Text style={[styles.value, { color: theme.foreground }]}>
                {formatUsageTotal(row.kind, row.total)}
              </Text>
            </View>
          ))}
        </View>
      ) : (
        <Text style={[styles.note, { color: theme.mutedForeground }]}>
          {glance.state === "healthy-empty" ? copy.empty : copy.noEvidence}
        </Text>
      )}

      {glance.through ? (
        <Text style={[styles.note, { color: theme.mutedForeground }]}>
          {copy.through(glance.through)}
        </Text>
      ) : null}
      {glance.degradedSources.length ? (
        <Text style={[styles.note, { color: theme.warning }]}>
          {copy.degraded(glance.degradedSources.join(", "))}
        </Text>
      ) : null}
      {glance.refreshUnavailable ? (
        <Text
          accessibilityRole="alert"
          style={[styles.note, { color: theme.warning }]}
        >
          {copy.refreshUnavailable}
        </Text>
      ) : null}
    </DashboardCard>
  );
}

const styles = StyleSheet.create({
  stateRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
    paddingBottom: space.sm,
  },
  period: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  state: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
  totalRow: {
    minHeight: 44,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.sm,
  },
  label: { flex: 1, fontSize: fontSizes.sm },
  value: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  note: { fontSize: fontSizes.xs, marginTop: space.sm },
});

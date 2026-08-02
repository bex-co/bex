import { router } from "expo-router";
import type { ReactNode } from "react";
import {
  ActivityIndicator,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import {
  SafeActionPanel,
  type MobileActionOption,
} from "@/components/safe-action";
import { useTranslations } from "@/common/hooks/use-translations";
import { fonts, fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { statusTone } from "./resource-groups";

export function DatastoreDetailLayout({
  title,
  subtitle,
  status,
  details,
  loading,
  error,
  refreshing,
  onRefresh,
  options,
  children,
}: {
  title: string;
  subtitle: string;
  status: string;
  details: Array<{
    label: string;
    value: string | null | undefined;
    mono?: boolean;
  }>;
  loading: boolean;
  error: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  options: MobileActionOption[];
  children?: ReactNode;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const tone = statusTone(status);
  const statusColor =
    tone === "success"
      ? theme.success
      : tone === "warning"
        ? theme.warning
        : tone === "error"
          ? theme.error
          : theme.mutedForeground;
  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <DashboardScrollView
        refreshing={refreshing}
        onRefresh={onRefresh}
        contentContainerStyle={styles.content}
      >
        <View style={styles.header}>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={t("common.backToStatus")}
            hitSlop={12}
            onPress={() => router.back()}
            style={styles.back}
          >
            <Ionicons name="chevron-back" size={26} color={theme.primary} />
          </Pressable>
          <View style={styles.headerCopy}>
            <Text style={[styles.title, { color: theme.foreground }]}>
              {title}
            </Text>
            <Text style={[styles.subtitle, { color: theme.mutedForeground }]}>
              {subtitle}
            </Text>
          </View>
        </View>
        {error ? (
          <View
            accessibilityRole="alert"
            style={[styles.notice, { borderColor: theme.warning }]}
          >
            <Text style={{ color: theme.warning }}>
              {t("datastores.loadError")}
            </Text>
          </View>
        ) : null}
        {loading ? (
          <DashboardCard>
            <ActivityIndicator color={theme.primary} style={styles.loading} />
          </DashboardCard>
        ) : (
          <>
            <DashboardCard title={t("datastores.overview")}>
              <View style={styles.statusRow}>
                <View style={[styles.dot, { backgroundColor: statusColor }]} />
                <Text style={[styles.status, { color: statusColor }]}>
                  {status}
                </Text>
              </View>
              {details.map((detail) => (
                <View
                  key={detail.label}
                  style={[styles.detail, { borderTopColor: theme.border }]}
                >
                  <Text style={{ color: theme.mutedForeground }}>
                    {detail.label}
                  </Text>
                  <Text
                    style={[
                      styles.detailValue,
                      { color: theme.foreground },
                      detail.mono ? { fontFamily: fonts.mono } : null,
                    ]}
                  >
                    {detail.value || "—"}
                  </Text>
                </View>
              ))}
            </DashboardCard>
            {children}
            <DashboardCard title={t("safeActions.cardTitle")}>
              <SafeActionPanel options={options} />
            </DashboardCard>
          </>
        )}
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { paddingTop: space.lg, paddingBottom: space.xxl },
  header: { flexDirection: "row", alignItems: "center", gap: space.sm },
  back: {
    width: 40,
    minHeight: 44,
    alignItems: "center",
    justifyContent: "center",
  },
  headerCopy: { flex: 1 },
  title: { fontSize: fontSizes.xxl, fontWeight: "700" },
  subtitle: { fontSize: fontSizes.sm, marginTop: space.xs },
  notice: { borderWidth: 1, borderRadius: space.sm, padding: space.md },
  loading: { minHeight: 120 },
  statusRow: { flexDirection: "row", alignItems: "center", gap: space.sm },
  dot: { width: 10, height: 10, borderRadius: 5 },
  status: { fontSize: fontSizes.lg, fontWeight: fontWeights.medium },
  detail: {
    flexDirection: "row",
    justifyContent: "space-between",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.sm,
  },
  detailValue: { flex: 1, textAlign: "right" },
});

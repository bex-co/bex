import type { ReactNode } from "react";
import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { DetailHeader } from "@/components/detail-header";
import {
  SafeActionPanel,
  type MobileActionOption,
} from "@/components/safe-action";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, fonts, space, useTheme } from "@/common/theme";
import { StatusBadge } from "./status-badge";

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
  return (
    <SafeAreaView
      edges={["top", "bottom", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DetailHeader title={title} subtitle={subtitle} />
      <DashboardScrollView
        refreshing={refreshing}
        onRefresh={onRefresh}
        contentContainerStyle={styles.content}
      >
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
              <StatusBadge status={status} />
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
  content: { gap: space.md },
  notice: { borderWidth: 1, borderRadius: space.sm, padding: space.md },
  loading: { minHeight: 120 },
  detail: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.sm,
  },
  detailValue: {
    flex: 1,
    textAlign: "right",
    fontSize: fontSizes.sm,
    flexShrink: 1,
  },
});

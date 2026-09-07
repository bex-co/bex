import { StyleSheet, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router } from "expo-router";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { DetailHeader } from "@/components/detail-header";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, space, useTheme } from "@/common/theme";
import { useCapabilities } from "./capabilities-provider";
import type { CapabilityAction } from "./capability-policy";

// ADR087: the generic presentation for a destination the caller cannot open,
// shared by every gated surface so the copy cannot fork: a confirmed denial
// reads as "not available with your access", an unresolved check as the
// neutral checking/unavailable states — never an echo of the resource name,
// and never a distinction between a foreign id and a nonexistent one.
export function AccessRequiredCard({ action }: { action: CapabilityAction }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const capabilities = useCapabilities();
  const bodyKey = capabilities.denied(action)
    ? "access.cannotOpen"
    : capabilities.state.status === "checking"
      ? "access.checking"
      : "access.unavailable";
  return (
    <DashboardCard>
      <Text
        accessibilityRole="alert"
        style={[styles.body, { color: theme.mutedForeground }]}
      >
        {t(bodyKey)}
      </Text>
      <Button
        type="outline"
        style={{ marginTop: space.lg }}
        onPress={() => router.replace("/")}
      >
        {t("access.backToStatus")}
      </Button>
    </DashboardCard>
  );
}

// AccessRequiredScreen is the pushed-detail-route shell around the card
// (session detail, logs); tab screens with their own chrome embed
// AccessRequiredCard directly.
export function AccessRequiredScreen({ action }: { action: CapabilityAction }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <SafeAreaView
      edges={["top", "bottom", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DetailHeader title={t("access.restrictedTitle")} subtitle="" />
      <DashboardScrollView contentContainerStyle={styles.content}>
        <AccessRequiredCard action={action} />
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { gap: space.lg },
  body: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
});

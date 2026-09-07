import { useMemo } from "react";
import { useLocalSearchParams } from "expo-router";
import { DetailHeader } from "@/components/detail-header";
import { StyleSheet } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { mobileConfig } from "@/features/auth/config";
import { authManager } from "@/features/auth/auth-provider";
import { AccessRequiredScreen } from "@/features/capabilities/access-required-screen";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validServiceDeepLink } from "@/features/navigation/deep-link";
import { LogSession, LogViewer, RestLogTransport } from "@/features/logs";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/theme";

export default function ServiceLogsScreen() {
  const { serviceId } = useLocalSearchParams<{
    serviceId?: string | string[];
  }>();
  const capabilities = useCapabilities();
  if (!validServiceDeepLink(serviceId)) return <InvalidDeepLinkScreen />;
  // ADR087: logs require confirmed can_view_logs. The gate sits ABOVE the
  // session construction, so neither the history request nor the SSE stream
  // ever mounts for a Viewer/Billing caller (or before access resolves).
  if (!capabilities.allows("can_view_logs")) {
    return <AccessRequiredScreen action="can_view_logs" />;
  }
  return <ValidatedServiceLogsScreen serviceId={serviceId} />;
}

function ValidatedServiceLogsScreen({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const session = useMemo(
    () =>
      new LogSession(
        new RestLogTransport(mobileConfig.apiOrigin, authManager),
        500,
      ),
    [],
  );
  return (
    <SafeAreaView
      edges={["top", "bottom", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DetailHeader title={t("logs.title")} subtitle={serviceId} />
      <LogViewer resource={serviceId} session={session} initialType="app" />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({ safe: { flex: 1 } });

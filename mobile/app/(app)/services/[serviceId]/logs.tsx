import { useMemo } from "react";
import { useLocalSearchParams, router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { mobileConfig } from "@/features/auth/config";
import { authManager } from "@/features/auth/auth-provider";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validServiceDeepLink } from "@/features/navigation/deep-link";
import { LogSession, LogViewer, RestLogTransport } from "@/features/logs";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, gutter, space, useTheme } from "@/common/theme";

export default function ServiceLogsScreen() {
  const { serviceId } = useLocalSearchParams<{
    serviceId?: string | string[];
  }>();
  if (!validServiceDeepLink(serviceId)) return <InvalidDeepLinkScreen />;
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
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <View style={[styles.header, { borderBottomColor: theme.border }]}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={t("logs.back")}
          hitSlop={12}
          onPress={() => router.back()}
          style={styles.back}
        >
          <Ionicons name="chevron-back" size={26} color={theme.primary} />
        </Pressable>
        <View style={styles.copy}>
          <Text style={[styles.title, { color: theme.foreground }]}>
            {t("logs.title")}
          </Text>
          <Text
            numberOfLines={1}
            style={[styles.resource, { color: theme.mutedForeground }]}
          >
            {serviceId}
          </Text>
        </View>
      </View>
      <LogViewer resource={serviceId} session={session} initialType="app" />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  header: {
    minHeight: 64,
    paddingHorizontal: gutter,
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  back: {
    width: 44,
    height: 44,
    alignItems: "center",
    justifyContent: "center",
  },
  copy: { flex: 1, minWidth: 0 },
  title: { fontSize: fontSizes.xl, fontWeight: "700" },
  resource: { fontSize: fontSizes.sm, marginTop: space.xxs },
});

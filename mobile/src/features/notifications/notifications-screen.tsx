import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { TopBar } from "@/components/top-bar";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatTimestamp } from "@/common/format-util";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { useNotifications } from "./notifications-provider";

export function NotificationInboxScreen() {
  const theme = useTheme().colorTheme;
  const { t, language } = useTranslations();
  const { state, items, unread, enable, disable, markAllRead, open } =
    useNotifications();
  const stateColor =
    state === "enabled"
      ? theme.success
      : state === "error" || state === "revoked"
        ? theme.error
        : theme.warning;
  const canDisable = state === "enabled" || state === "offline";
  const cardStyle = {
    backgroundColor: theme.card,
    borderColor: theme.border,
  };
  const unreadStyle = { borderLeftColor: theme.primary, borderLeftWidth: 4 };
  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <TopBar title={t("notifications.title")} showBell={false} />
      <DashboardScrollView
        refreshing={false}
        onRefresh={() => undefined}
        contentContainerStyle={styles.content}
      >
        <Text style={[styles.body, { color: theme.mutedForeground }]}>
          {t("notifications.body")}
        </Text>
        <DashboardCard
          title={t("notifications.settings")}
          right={
            <Button
              type={canDisable ? "outline" : "primary"}
              style={styles.action}
              loading={state === "enabling" || state === "checking"}
              onPress={() => void (canDisable ? disable() : enable())}
            >
              {t(canDisable ? "notifications.disable" : "notifications.enable")}
            </Button>
          }
          style={styles.settingsCard}
        >
          <Text style={[styles.state, { color: stateColor }]}>
            {t(`notifications.states.${state}`)}
          </Text>
          <Text style={[styles.body, { color: theme.mutedForeground }]}>
            {t("notifications.permissionExplanation")}
          </Text>
        </DashboardCard>
        <View style={styles.row}>
          <Text style={[styles.sectionTitle, { color: theme.foreground }]}>
            {t("notifications.recent", { count: items.length })}
          </Text>
          {unread > 0 ? (
            <Pressable
              accessibilityRole="button"
              onPress={() => void markAllRead()}
            >
              <Text style={[styles.link, { color: theme.primary }]}>
                {t("notifications.markAllRead")}
              </Text>
            </Pressable>
          ) : null}
        </View>
        {items.length === 0 ? (
          <Text style={[styles.empty, { color: theme.mutedForeground }]}>
            {t("notifications.empty")}
          </Text>
        ) : null}
        {items.map((item) => (
          <Pressable
            key={item.id}
            accessibilityRole="button"
            accessibilityLabel={t("notifications.open", { title: item.title })}
            onPress={() => void open(item)}
            style={[styles.item, cardStyle, !item.read && unreadStyle]}
          >
            <Text style={[styles.itemTitle, { color: theme.foreground }]}>
              {item.title}
            </Text>
            {item.body ? (
              <Text style={[styles.body, { color: theme.mutedForeground }]}>
                {item.body}
              </Text>
            ) : null}
            <Text style={[styles.timestamp, { color: theme.mutedForeground }]}>
              {formatTimestamp(item.receivedAt, language)}
            </Text>
          </Pressable>
        ))}
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { paddingTop: space.sm, paddingBottom: space.xxl, gap: space.md },
  body: { fontSize: fontSizes.md, lineHeight: 21 },
  settingsCard: { marginBottom: 0 },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
  },
  sectionTitle: { fontSize: fontSizes.lg, fontWeight: fontWeights.medium },
  state: { fontSize: fontSizes.sm, marginBottom: space.sm },
  action: { minWidth: 112, paddingHorizontal: space.lg, flexShrink: 0 },
  item: {
    borderWidth: 1,
    borderRadius: gutter,
    padding: space.lg,
    gap: space.sm,
  },
  itemTitle: { fontSize: fontSizes.lg, fontWeight: fontWeights.medium },
  timestamp: { fontSize: fontSizes.sm },
  empty: { textAlign: "center", paddingVertical: space.xxl },
  link: { fontWeight: fontWeights.medium },
});

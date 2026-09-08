import { Pressable, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { SafeAreaView } from "react-native-safe-area-context";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { TopBar } from "@/components/top-bar";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatTimestamp } from "@/common/format-util";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { useNotifications } from "./notifications-provider";

export function NotificationInboxScreen() {
  const theme = useTheme().colorTheme;
  const { t, language } = useTranslations();
  const {
    state,
    items,
    unread,
    inboxState,
    retry,
    enable,
    disable,
    markAllRead,
    open,
  } = useNotifications();
  const stateColor =
    state === "enabled"
      ? theme.success
      : state === "error" || state === "revoked"
        ? theme.error
        : theme.warning;
  const configurable = state !== "unconfigured" && state !== "unavailable";
  const canDisable = state === "enabled" || state === "offline";
  const cardStyle = {
    backgroundColor: theme.card,
    borderColor: theme.border,
  };
  const unreadStyle = {
    borderLeftColor: theme.primary,
    borderLeftWidth: 3,
  };
  return (
    <SafeAreaView
      edges={["top", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DashboardScrollView
        header={<TopBar />}
        contentContainerStyle={styles.content}
      >
        <Text style={[styles.body, { color: theme.mutedForeground }]}>
          {t("notifications.body")}
        </Text>
        {items.length > 0 ? (
          <View style={styles.row}>
            <Text style={[styles.sectionTitle, { color: theme.foreground }]}>
              {t("notifications.recent", { count: items.length })}
            </Text>
            {unread > 0 ? (
              <Pressable
                accessibilityRole="button"
                style={styles.markRead}
                onPress={() => void markAllRead()}
              >
                <Text style={[styles.link, { color: theme.primary }]}>
                  {t("notifications.markAllRead")}
                </Text>
              </Pressable>
            ) : null}
          </View>
        ) : null}
        {inboxState !== "ready" ? (
          <DashboardCard>
            <Text
              accessibilityLiveRegion="polite"
              style={[styles.body, { color: theme.mutedForeground }]}
            >
              {t(
                inboxState === "checking"
                  ? "notifications.inboxChecking"
                  : "notifications.inboxError",
              )}
            </Text>
            {inboxState === "error" ? (
              <Button
                type="outline"
                onPress={() => void retry().catch(() => undefined)}
              >
                {t("auth.retry")}
              </Button>
            ) : null}
          </DashboardCard>
        ) : null}
        {items.length === 0 && inboxState === "ready" ? (
          <DashboardCard>
            <View style={styles.empty}>
              <View
                style={[
                  styles.emptyIcon,
                  { backgroundColor: theme.primaryMuted },
                ]}
              >
                <Ionicons
                  name="notifications-outline"
                  size={28}
                  color={theme.primary}
                />
              </View>
              <Text style={[styles.emptyTitle, { color: theme.foreground }]}>
                {t("notifications.emptyTitle")}
              </Text>
              <Text
                style={[styles.emptyBody, { color: theme.mutedForeground }]}
              >
                {t("notifications.empty")}
              </Text>
            </View>
          </DashboardCard>
        ) : null}
        {items.map((item) => (
          <Pressable
            key={item.id}
            accessibilityRole="button"
            accessibilityLabel={t("notifications.open", { title: item.title })}
            onPress={() => void open(item)}
            style={({ pressed }) => [
              styles.item,
              cardStyle,
              !item.read && unreadStyle,
              { opacity: pressed ? 0.65 : 1 },
            ]}
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
        <DashboardCard
          title={t("notifications.settings")}
          right={
            configurable ? (
              <Button
                type={canDisable ? "outline" : "primary"}
                style={styles.action}
                loading={state === "enabling" || state === "checking"}
                onPress={() => void (canDisable ? disable() : enable())}
              >
                {t(
                  canDisable ? "notifications.disable" : "notifications.enable",
                )}
              </Button>
            ) : null
          }
        >
          <Text style={[styles.state, { color: stateColor }]}>
            {t(`notifications.states.${state}`)}
          </Text>
          <Text style={[styles.body, { color: theme.mutedForeground }]}>
            {t(
              configurable
                ? "notifications.permissionExplanation"
                : "notifications.benefit",
            )}
          </Text>
        </DashboardCard>
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { gap: space.lg },
  body: { fontSize: fontSizes.md, lineHeight: fontSizes.md * 1.5 },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: space.md,
  },
  sectionTitle: {
    flex: 1,
    fontSize: fontSizes.lg,
    fontWeight: fontWeights.semibold,
  },
  state: { fontSize: fontSizes.sm, marginBottom: space.sm },
  action: { minWidth: 112, paddingHorizontal: space.lg, flexShrink: 0 },
  item: {
    borderWidth: 1,
    borderRadius: space.md,
    padding: space.md,
    gap: space.sm,
    overflow: "hidden",
  },
  itemTitle: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  timestamp: { fontSize: fontSizes.xs },
  empty: { alignItems: "center", paddingVertical: space.xl, gap: space.md },
  emptyTitle: {
    fontSize: fontSizes.xl,
    fontWeight: fontWeights.semibold,
    textAlign: "center",
  },
  emptyBody: { fontSize: fontSizes.md, lineHeight: 22, textAlign: "center" },
  emptyIcon: {
    width: 56,
    height: 56,
    borderRadius: space.lg,
    justifyContent: "center",
    alignItems: "center",
  },
  markRead: { minHeight: 44, justifyContent: "center", flexShrink: 1 },
  link: { fontWeight: fontWeights.medium },
});

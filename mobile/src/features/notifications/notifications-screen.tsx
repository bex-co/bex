import {
  Pressable,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from "react-native";
import { Button } from "@/components/button";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { AppDrawerButton } from "@/components/app-drawer";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { useNotifications } from "./notifications-provider";

export function NotificationInboxScreen() {
  const { width } = useWindowDimensions();
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { state, items, unread, enable, disable, markAllRead, open } =
    useNotifications();
  const styles = StyleSheet.create({
    content: { paddingTop: space.sm, paddingBottom: gutter * 2, gap: space.md },
    titleRow: { flexDirection: "row", alignItems: "center", gap: space.sm },
    title: {
      color: theme.foreground,
      fontSize: fontSizes.xxl,
      fontWeight: fontWeights.medium,
    },
    body: {
      color: theme.mutedForeground,
      fontSize: fontSizes.md,
      lineHeight: 21,
    },
    card: {
      backgroundColor: theme.card,
      borderColor: theme.border,
      borderWidth: 1,
      borderRadius: 12,
      padding: space.lg,
      gap: space.md,
    },
    row: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      gap: space.md,
    },
    sectionTitle: {
      color: theme.foreground,
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.medium,
    },
    state: {
      color:
        state === "enabled"
          ? theme.success
          : state === "error" || state === "revoked"
            ? theme.error
            : theme.warning,
      fontSize: fontSizes.md,
    },
    action: {
      minWidth: Math.min(180, width * 0.44),
      paddingHorizontal: space.md,
    },
    item: {
      backgroundColor: theme.card,
      borderColor: theme.border,
      borderWidth: 1,
      borderRadius: 12,
      padding: space.lg,
      gap: space.sm,
    },
    unread: { borderLeftColor: theme.primary, borderLeftWidth: 4 },
    itemTitle: {
      color: theme.foreground,
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.medium,
    },
    timestamp: { color: theme.mutedForeground, fontSize: fontSizes.sm },
    empty: {
      color: theme.mutedForeground,
      textAlign: "center",
      paddingVertical: space.xxl,
    },
    link: { color: theme.primary, fontWeight: fontWeights.medium },
  });
  const canDisable = state === "enabled" || state === "offline";
  return (
    <DashboardScrollView
      refreshing={false}
      onRefresh={() => undefined}
      contentContainerStyle={styles.content}
    >
      <View style={styles.titleRow}>
        <AppDrawerButton />
        <Text style={styles.title}>{t("notifications.title")}</Text>
      </View>
      <Text style={styles.body}>{t("notifications.body")}</Text>
      <View style={styles.card}>
        <View style={styles.row}>
          <View>
            <Text style={styles.sectionTitle}>
              {t("notifications.settings")}
            </Text>
            <Text style={styles.state}>
              {t(`notifications.states.${state}`)}
            </Text>
          </View>
          <Button
            type={canDisable ? "outline" : "primary"}
            style={styles.action}
            loading={state === "enabling" || state === "checking"}
            onPress={() => void (canDisable ? disable() : enable())}
          >
            {t(canDisable ? "notifications.disable" : "notifications.enable")}
          </Button>
        </View>
        <Text style={styles.body}>
          {t("notifications.permissionExplanation")}
        </Text>
      </View>
      <View style={styles.row}>
        <Text style={styles.sectionTitle}>
          {t("notifications.recent", { count: items.length })}
        </Text>
        {unread > 0 ? (
          <Pressable
            accessibilityRole="button"
            onPress={() => void markAllRead()}
          >
            <Text style={styles.link}>{t("notifications.markAllRead")}</Text>
          </Pressable>
        ) : null}
      </View>
      {items.length === 0 ? (
        <Text style={styles.empty}>{t("notifications.empty")}</Text>
      ) : null}
      {items.map((item) => (
        <Pressable
          key={item.id}
          accessibilityRole="button"
          accessibilityLabel={t("notifications.open", { title: item.title })}
          onPress={() => void open(item)}
          style={[styles.item, !item.read && styles.unread]}
        >
          <Text style={styles.itemTitle}>{item.title}</Text>
          {item.body ? <Text style={styles.body}>{item.body}</Text> : null}
          <Text style={styles.timestamp}>
            {new Date(item.receivedAt).toLocaleString()}
          </Text>
        </Pressable>
      ))}
    </DashboardScrollView>
  );
}

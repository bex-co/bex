import type { ReactNode } from "react";
import { StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { router } from "expo-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { useNotifications } from "@/features/notifications/notifications-provider";
import {
  fontSizes,
  fontWeights,
  gutter,
  maxFontSizeMultipliers,
  space,
  useTheme,
} from "@/common/theme";
import { AppDrawerButton } from "@/components/app-drawer";

function NotificationsBellButton() {
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { unread } = useNotifications();
  return (
    <TouchableOpacity
      accessibilityRole="button"
      accessibilityLabel={t("notifications.title")}
      onPress={() => router.navigate("/notifications")}
      hitSlop={8}
      activeOpacity={0.7}
    >
      <View>
        <Ionicons
          name="notifications-outline"
          size={24}
          color={theme.foreground}
        />
        {unread > 0 ? (
          <View style={[styles.badge, { backgroundColor: theme.error }]}>
            <Text style={[styles.badgeText, { color: theme.white }]}>
              {unread > 99 ? "99+" : unread}
            </Text>
          </View>
        ) : null}
      </View>
    </TouchableOpacity>
  );
}

/**
 * Slim fixed header for top-level tab screens: equal-width action areas keep
 * the title truly centered. Left hosts the shared drawer trigger and the
 * notifications bell; `right` is a screen-specific slot.
 */
export function TopBar({
  title,
  right,
  showBell = true,
  showDrawer = true,
}: {
  title: string;
  right?: ReactNode;
  showBell?: boolean;
  showDrawer?: boolean;
}) {
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.bar}>
      <View style={styles.side}>
        {showDrawer ? <AppDrawerButton /> : null}
        {showBell ? <NotificationsBellButton /> : null}
      </View>
      <Text
        numberOfLines={1}
        maxFontSizeMultiplier={maxFontSizeMultipliers.heading}
        style={[styles.title, { color: theme.foreground }]}
      >
        {title}
      </Text>
      <View style={[styles.side, styles.right]}>{right}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: gutter,
    paddingVertical: space.xs,
  },
  side: {
    width: 80,
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
  },
  right: { justifyContent: "flex-end" },
  title: {
    flex: 1,
    fontSize: fontSizes.xl,
    fontWeight: fontWeights.medium,
    textAlign: "center",
  },
  badge: {
    position: "absolute",
    top: -4,
    right: -6,
    minWidth: 16,
    height: 16,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    paddingHorizontal: 2,
  },
  // A count inside a 16pt dot needs to sit below the fontSizes scale.
  badgeText: {
    fontSize: 10,
    fontWeight: fontWeights.medium,
    lineHeight: 14,
  },
});

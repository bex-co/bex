import { Tabs } from "expo-router";
import { NativeTabs } from "expo-router/unstable-native-tabs";
import {
  Platform,
  StyleSheet,
  Text,
  View,
  type ColorValue,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { fontSizes, fontWeights, useTheme, withAlpha } from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";
import { HapticTab } from "@/components/haptic-tab";
import { TabBarIcon } from "@/components/tab-bar-icon";
import { nativeTabIcons } from "@/components/tab-bar-icon/tab-icons";
import { useNotifications } from "@/features/notifications/notifications-provider";

const TAB_BAR_CONTENT_HEIGHT = 80;
const TAB_BAR_PILL_INSET = 6;
const TAB_BAR_SIDE_INSET = 12;

const styles = StyleSheet.create({
  label: {
    fontSize: fontSizes.xs,
    fontWeight: fontWeights.medium,
    lineHeight: fontSizes.xs + 2,
    marginTop: 2,
  },
  background: {
    position: "absolute",
    borderRadius: 36,
    borderCurve: "continuous",
    borderWidth: StyleSheet.hairlineWidth,
    shadowOffset: { width: 0, height: 5 },
    shadowRadius: 18,
    elevation: 10,
  },
});

function renderTabBarLabel({
  children,
  color,
}: {
  children: string;
  color: ColorValue;
}) {
  return (
    <Text
      adjustsFontSizeToFit
      minimumFontScale={0.78}
      numberOfLines={1}
      style={[styles.label, { color }]}
    >
      {children}
    </Text>
  );
}

export default function TabLayout() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const insets = useSafeAreaInsets();
  const { unread } = useNotifications();

  if (Platform.OS === "ios") {
    const contentStyle = { backgroundColor: theme.background };
    return (
      // UIKit owns the material, sizing, typography, and selection animation.
      // iOS 26 gets Liquid Glass; earlier versions keep their native bar.
      <NativeTabs
        tintColor={theme.primary}
        minimizeBehavior="onScrollDown"
        backBehavior="history"
      >
        <NativeTabs.Trigger name="index" contentStyle={contentStyle}>
          <NativeTabs.Trigger.Label>
            {t("navigation.status")}
          </NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={nativeTabIcons.index} />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="activity" contentStyle={contentStyle}>
          <NativeTabs.Trigger.Label>
            {t("navigation.activity")}
          </NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={nativeTabIcons.activity} />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="sessions" contentStyle={contentStyle}>
          <NativeTabs.Trigger.Label>
            {t("navigation.sessions")}
          </NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={nativeTabIcons.sessions} />
        </NativeTabs.Trigger>
        <NativeTabs.Trigger name="notifications" contentStyle={contentStyle}>
          <NativeTabs.Trigger.Label>
            {t("navigation.notifications")}
          </NativeTabs.Trigger.Label>
          <NativeTabs.Trigger.Icon sf={nativeTabIcons.notifications} />
          {unread > 0 ? (
            <NativeTabs.Trigger.Badge>
              {String(unread)}
            </NativeTabs.Trigger.Badge>
          ) : null}
        </NativeTabs.Trigger>
      </NativeTabs>
    );
  }

  return (
    <Tabs
      initialRouteName="index"
      backBehavior="history"
      screenOptions={{
        headerShown: false,
        lazy: true,
        tabBarButton: HapticTab,
        tabBarActiveTintColor: theme.primary,
        tabBarInactiveTintColor: theme.mutedForeground,
        tabBarShowLabel: true,
        tabBarLabelPosition: "below-icon",
        tabBarLabel: renderTabBarLabel,
        tabBarInactiveBackgroundColor: "transparent",
        tabBarItemStyle: {
          minWidth: 0,
          marginHorizontal: 3,
          marginVertical: 7,
        },
        tabBarBadgeStyle: {
          backgroundColor: theme.error,
          color: theme.isDark ? theme.background : theme.card,
        },
        tabBarStyle: {
          position: "absolute",
          backgroundColor: "transparent",
          borderTopWidth: 0,
          elevation: 0,
          paddingHorizontal: TAB_BAR_SIDE_INSET,
          height: TAB_BAR_CONTENT_HEIGHT + insets.bottom,
        },
        tabBarBackground: () => (
          <View
            pointerEvents="none"
            style={[
              StyleSheet.absoluteFill,
              styles.background,
              {
                top: TAB_BAR_PILL_INSET,
                right: TAB_BAR_SIDE_INSET,
                bottom: insets.bottom + TAB_BAR_PILL_INSET,
                left: TAB_BAR_SIDE_INSET,
                backgroundColor: withAlpha(
                  theme.card,
                  theme.isDark ? 0.94 : 0.9,
                ),
                borderColor: withAlpha(theme.border, 0.6),
                shadowColor: theme.nav01,
                shadowOpacity: theme.isDark ? 0.4 : 0.16,
              },
            ]}
          />
        ),
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: t("navigation.status"),
          tabBarIcon: ({ color, focused }) => (
            <TabBarIcon route="index" color={color} focused={focused} />
          ),
        }}
      />
      <Tabs.Screen
        name="activity"
        options={{
          title: t("navigation.activity"),
          tabBarIcon: ({ color, focused }) => (
            <TabBarIcon route="activity" color={color} focused={focused} />
          ),
        }}
      />
      <Tabs.Screen
        name="sessions"
        options={{
          title: t("navigation.sessions"),
          tabBarIcon: ({ color, focused }) => (
            <TabBarIcon route="sessions" color={color} focused={focused} />
          ),
        }}
      />
      <Tabs.Screen
        name="notifications"
        options={{
          title: t("navigation.notifications"),
          tabBarBadge: unread > 0 ? unread : undefined,
          tabBarIcon: ({ color, focused }) => (
            <TabBarIcon route="notifications" color={color} focused={focused} />
          ),
        }}
      />
    </Tabs>
  );
}

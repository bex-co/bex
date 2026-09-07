import { ReactNode, useContext } from "react";
import { BottomTabBarHeightContext } from "expo-router/build/react-navigation/bottom-tabs";
import {
  RefreshControl,
  ScrollView,
  StyleProp,
  StyleSheet,
  View,
  ViewStyle,
} from "react-native";
import { gutter, space, useTheme } from "@/common/theme";

const styles = StyleSheet.create({
  fill: { flexGrow: 1 },
  content: {
    // Side gutters keep cards off the screen edges — the app-wide inset.
    // The container owns spacing; cards do not add a second bottom margin.
    paddingHorizontal: gutter,
    paddingTop: space.md,
    paddingBottom: space.xxl,
    gap: space.lg,
    // Fill the frame even when the cards are short, so the whole area stays
    // inside the scrollable content and pull-to-refresh works everywhere.
    flexGrow: 1,
  },
});

type Props = {
  /** Stays at the top while keeping the scroll view first in the native tree. */
  header?: ReactNode;
  refreshing?: boolean;
  onRefresh?: () => void;
  /** Extra content-container styles (merged after the shared gutters). */
  contentContainerStyle?: StyleProp<ViewStyle>;
  children: ReactNode;
};

/**
 * Shared vertical scroll container for dashboard-style screens (Home, Reports):
 * consistent gutters, a dark-aware scroll indicator, and pull-to-refresh wired
 * to the screen's refresh state. Keeps the screens visually consistent and free
 * of duplicated ScrollView/RefreshControl boilerplate.
 */
export function DashboardScrollView({
  header,
  refreshing = false,
  onRefresh,
  contentContainerStyle,
  children,
}: Props): React.ReactElement {
  const theme = useTheme().colorTheme;
  const isDark = theme.isDark;
  // Only JS tabs provide a height. Native iOS tabs adjust their scroll insets
  // automatically, and detail screens live outside the tab navigator.
  const tabBarHeight = useContext(BottomTabBarHeightContext) ?? 0;
  const contentStyle = StyleSheet.flatten([
    styles.content,
    contentContainerStyle,
  ]);
  const paddedContentStyle = [
    contentStyle,
    { paddingBottom: Number(contentStyle.paddingBottom ?? 0) + tabBarHeight },
  ];

  return (
    <ScrollView
      showsVerticalScrollIndicator={false}
      alwaysBounceVertical
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="on-drag"
      stickyHeaderIndices={header ? [0] : undefined}
      contentContainerStyle={header ? styles.fill : paddedContentStyle}
      indicatorStyle={isDark ? "white" : "default"}
      refreshControl={
        onRefresh ? (
          <RefreshControl
            refreshing={refreshing}
            onRefresh={onRefresh}
            tintColor={isDark ? "white" : "black"}
          />
        ) : undefined
      }
    >
      {header ? (
        <View style={{ backgroundColor: theme.background }}>{header}</View>
      ) : null}
      {header ? <View style={paddedContentStyle}>{children}</View> : children}
    </ScrollView>
  );
}

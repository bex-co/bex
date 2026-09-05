import { ReactNode } from "react";
import {
  RefreshControl,
  ScrollView,
  StyleProp,
  StyleSheet,
  ViewStyle,
} from "react-native";
import { gutter, space, useTheme } from "@/common/theme";

const styles = StyleSheet.create({
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
  refreshing = false,
  onRefresh,
  contentContainerStyle,
  children,
}: Props): React.ReactElement {
  const isDark = useTheme().colorTheme.isDark;

  return (
    <ScrollView
      showsVerticalScrollIndicator={false}
      alwaysBounceVertical
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="on-drag"
      contentContainerStyle={[styles.content, contentContainerStyle]}
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
      {children}
    </ScrollView>
  );
}

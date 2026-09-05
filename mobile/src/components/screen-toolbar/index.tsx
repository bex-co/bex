import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";

/** Equal action slots keep detail and sheet titles centered, even with one action. */
export function ScreenToolbar({
  title,
  subtitle,
  left,
  right,
  textActions = false,
}: {
  title?: string;
  subtitle?: string;
  left?: ReactNode;
  right?: ReactNode;
  textActions?: boolean;
}) {
  const theme = useTheme().colorTheme;
  return (
    <View style={[styles.bar, { borderBottomColor: theme.border }]}>
      <View style={[styles.side, textActions && styles.textSide]}>{left}</View>
      <View style={styles.copy}>
        {title ? (
          <Text
            accessibilityRole="header"
            style={[styles.title, { color: theme.foreground }]}
          >
            {title}
          </Text>
        ) : null}
        {subtitle ? (
          <Text
            numberOfLines={1}
            style={[styles.subtitle, { color: theme.mutedForeground }]}
          >
            {subtitle}
          </Text>
        ) : null}
      </View>
      <View style={[styles.side, styles.right, textActions && styles.textSide]}>
        {right}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: "row",
    alignItems: "center",
    minHeight: 60,
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
    gap: space.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  side: {
    width: 44,
    minHeight: 44,
    justifyContent: "center",
    alignItems: "flex-start",
  },
  textSide: { width: 64 },
  right: { alignItems: "flex-end" },
  copy: { flex: 1, minWidth: 0, gap: space.xxs },
  title: {
    fontSize: fontSizes.xl,
    fontWeight: fontWeights.semibold,
    textAlign: "center",
  },
  subtitle: { fontSize: fontSizes.sm, textAlign: "center" },
});

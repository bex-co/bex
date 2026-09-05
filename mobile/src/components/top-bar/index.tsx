import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { AppDrawerButton } from "@/components/app-drawer";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/workspace-provider";

/** The workspace menu, current workspace, and page action in one compact row. */
export function TopBar({ right }: { right?: ReactNode }) {
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { selected } = useWorkspace();
  const title = selected?.name || t("drawer.workspaces");
  return (
    <View style={[styles.bar, { borderBottomColor: theme.border }]}>
      <AppDrawerButton />
      <Text
        accessibilityRole="header"
        accessibilityLabel={title}
        numberOfLines={1}
        style={[styles.title, { color: theme.foreground }]}
      >
        {title}
      </Text>
      {right ? <View style={styles.right}>{right}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
    minHeight: 60,
    gap: space.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
    flexDirection: "row",
    alignItems: "center",
  },
  title: {
    flex: 1,
    minWidth: 0,
    fontSize: fontSizes.title,
    fontWeight: fontWeights.semibold,
    letterSpacing: -0.3,
  },
  right: { flexShrink: 0, alignItems: "center", justifyContent: "center" },
});

import type { ComponentProps, ReactNode } from "react";
import { Ionicons } from "@expo/vector-icons";
import { SafeAreaView } from "react-native-safe-area-context";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/theme";
import { gutter, space } from "@/common/theme";
import { AppDrawerButton } from "@/components/app-drawer";

type IconName = ComponentProps<typeof Ionicons>["name"];

export function ShellScreen({
  titleKey,
  bodyKey,
  badgeKey,
  icon,
  menu = false,
  children,
}: {
  titleKey: string;
  bodyKey: string;
  badgeKey?: string;
  icon: IconName;
  /** Show the shared drawer trigger — set on top-level tab screens. */
  menu?: boolean;
  children?: ReactNode;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { width } = useWindowDimensions();
  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      {menu ? (
        <View style={styles.header}>
          <AppDrawerButton />
        </View>
      ) : null}
      <View style={[styles.content, { maxWidth: Math.min(width - 32, 680) }]}>
        <View style={[styles.icon, { backgroundColor: theme.primaryMuted }]}>
          <Ionicons name={icon} size={28} color={theme.primary} />
        </View>
        <Text style={[styles.title, { color: theme.foreground }]}>
          {t(titleKey)}
        </Text>
        <Text style={[styles.body, { color: theme.mutedForeground }]}>
          {t(bodyKey)}
        </Text>
        {badgeKey ? (
          <Text
            accessibilityRole="text"
            style={[
              styles.badge,
              { color: theme.primary, borderColor: theme.primary },
            ]}
          >
            {t(badgeKey)}
          </Text>
        ) : null}
        {children}
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  header: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: gutter,
    paddingTop: space.xs,
  },
  content: {
    flex: 1,
    alignSelf: "center",
    width: "100%",
    padding: 24,
    gap: 12,
  },
  icon: {
    width: 52,
    height: 52,
    borderRadius: 16,
    alignItems: "center",
    justifyContent: "center",
    marginTop: 24,
  },
  title: { fontSize: 28, fontWeight: "700", marginTop: 8 },
  body: { fontSize: 16, lineHeight: 24 },
  badge: {
    alignSelf: "flex-start",
    borderWidth: 1,
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 6,
    fontSize: 13,
    fontWeight: "600",
  },
});

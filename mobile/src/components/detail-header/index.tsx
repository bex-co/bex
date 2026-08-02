import { Pressable, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { router } from "expo-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, space, useTheme } from "@/common/theme";

/** Back-chevron header with truncating title/subtitle for detail screens. */
export function DetailHeader({
  title,
  subtitle,
}: {
  title: string;
  subtitle: string;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.header}>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={t("common.backToStatus")}
        hitSlop={12}
        onPress={() => router.back()}
        style={styles.back}
      >
        <Ionicons name="chevron-back" size={26} color={theme.primary} />
      </Pressable>
      <View style={styles.copy}>
        <Text
          numberOfLines={1}
          style={[styles.title, { color: theme.foreground }]}
        >
          {title}
        </Text>
        <Text
          numberOfLines={1}
          style={[styles.subtitle, { color: theme.mutedForeground }]}
        >
          {subtitle}
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  header: { flexDirection: "row", alignItems: "center", gap: space.sm },
  back: {
    width: 40,
    minHeight: 44,
    alignItems: "center",
    justifyContent: "center",
  },
  copy: { flex: 1 },
  title: { fontSize: fontSizes.xxl, fontWeight: "700" },
  subtitle: { fontSize: fontSizes.sm, marginTop: space.xs },
});

import { Pressable, StyleSheet } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { ScreenToolbar } from "@/components/screen-toolbar";
import { router } from "expo-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/theme";

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
    <ScreenToolbar
      title={title}
      subtitle={subtitle}
      left={
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={t("common.back")}
          onPress={() => router.back()}
          style={styles.back}
        >
          <Ionicons name="chevron-back" size={24} color={theme.foreground} />
        </Pressable>
      }
    />
  );
}

const styles = StyleSheet.create({
  back: {
    width: 44,
    minHeight: 44,
    alignItems: "center",
    justifyContent: "center",
  },
});

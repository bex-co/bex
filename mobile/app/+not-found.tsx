import { Link, Stack } from "expo-router";
import { StyleSheet, Text, View } from "react-native";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/theme";

export default function NotFound() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View style={[styles.container, { backgroundColor: theme.background }]}>
      <Stack.Screen options={{ title: t("common.notFoundTitle") }} />
      <Text style={[styles.title, { color: theme.foreground }]}>
        {t("common.notFoundTitle")}
      </Text>
      <Link href="/" style={[styles.link, { color: theme.primary }]}>
        {t("common.backToStatus")}
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 12,
  },
  title: { fontSize: 20, fontWeight: "600" },
  link: { fontSize: 16 },
});

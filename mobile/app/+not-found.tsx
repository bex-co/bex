import { Link, Stack } from "expo-router";
import { StyleSheet, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";

export default function NotFound() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <SafeAreaView
      style={[styles.container, { backgroundColor: theme.background }]}
    >
      <Stack.Screen options={{ title: t("common.notFoundTitle") }} />
      <Text style={[styles.title, { color: theme.foreground }]}>
        {t("common.notFoundTitle")}
      </Text>
      <Link href="/" style={[styles.link, { color: theme.primary }]}>
        {t("common.backToStatus")}
      </Link>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: space.md,
  },
  title: { fontSize: fontSizes.xl, fontWeight: fontWeights.medium },
  link: { fontSize: fontSizes.lg },
});

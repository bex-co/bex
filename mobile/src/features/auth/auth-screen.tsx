import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useTheme } from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/components/button";

export function AuthStateScreen({
  titleKey,
  bodyKey,
  actionKey,
  busy = false,
  onAction,
  secondaryActionKey,
  onSecondaryAction,
}: {
  titleKey: string;
  bodyKey: string;
  actionKey?: string;
  busy?: boolean;
  onAction?: () => void;
  secondaryActionKey?: string;
  onSecondaryAction?: () => void;
}) {
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <View style={styles.content} accessibilityRole="summary">
        {busy ? <ActivityIndicator color={theme.primary} size="large" /> : null}
        <Text style={[styles.title, { color: theme.foreground }]}>
          {t(titleKey)}
        </Text>
        <Text style={[styles.body, { color: theme.mutedForeground }]}>
          {t(bodyKey)}
        </Text>
        {actionKey && onAction ? (
          <Button
            style={styles.action}
            onPress={onAction}
            disabled={busy}
            loading={busy}
            accessibilityLabel={t(actionKey)}
          >
            {t(actionKey)}
          </Button>
        ) : null}
        {secondaryActionKey && onSecondaryAction ? (
          <Button
            type="outline"
            style={styles.action}
            onPress={onSecondaryAction}
            disabled={busy}
            accessibilityLabel={t(secondaryActionKey)}
          >
            {t(secondaryActionKey)}
          </Button>
        ) : null}
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: {
    flex: 1,
    justifyContent: "center",
    paddingHorizontal: 28,
    gap: 14,
    maxWidth: 560,
    width: "100%",
    alignSelf: "center",
  },
  title: { fontSize: 30, fontWeight: "700", textAlign: "center" },
  body: { fontSize: 16, lineHeight: 24, textAlign: "center" },
  action: { alignSelf: "stretch", marginTop: 12 },
});

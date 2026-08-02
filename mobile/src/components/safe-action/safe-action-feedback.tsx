import { Pressable, StyleSheet, Text, View } from "react-native";
import { fontSizes, space, useTheme } from "@/common/theme";
import type { ColorTheme } from "@/types/theme-props";
import type { SafeActionFeedback, SafeActionOutcome } from "./executor";

export type SafeActionFeedbackMessages = Record<SafeActionFeedback, string>;

export interface SafeActionFeedbackViewProps {
  outcome: SafeActionOutcome<unknown> | null;
  messages: SafeActionFeedbackMessages;
  retryLabel: string;
  dismissLabel: string;
  onRetry: () => void;
  onDismiss: () => void;
}

const stylesFor = (theme: ColorTheme) =>
  StyleSheet.create({
    container: {
      gap: space.sm,
      padding: space.md,
      borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.border,
      borderRadius: space.sm,
      backgroundColor: theme.card,
    },
    message: { color: theme.foreground, fontSize: fontSizes.md },
    actions: {
      flexDirection: "row",
      justifyContent: "flex-end",
      gap: space.md,
    },
    action: { minHeight: 44, justifyContent: "center" },
    actionText: { color: theme.primary, fontSize: fontSizes.md },
  });

/** Honest result surface: unknown/partial outcomes never render as success. */
export function SafeActionFeedbackView({
  outcome,
  messages,
  retryLabel,
  dismissLabel,
  onRetry,
  onDismiss,
}: SafeActionFeedbackViewProps) {
  const theme = useTheme().colorTheme;
  const styles = stylesFor(theme);
  if (!outcome) return null;
  return (
    <View accessibilityLiveRegion="polite" style={styles.container}>
      <Text style={styles.message}>{messages[outcome.feedback]}</Text>
      <View style={styles.actions}>
        {outcome.canRetry ? (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={retryLabel}
            onPress={onRetry}
            style={styles.action}
          >
            <Text style={styles.actionText}>{retryLabel}</Text>
          </Pressable>
        ) : null}
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={dismissLabel}
          onPress={onDismiss}
          style={styles.action}
        >
          <Text style={styles.actionText}>{dismissLabel}</Text>
        </Pressable>
      </View>
    </View>
  );
}

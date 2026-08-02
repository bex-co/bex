import {
  ActivityIndicator,
  Modal,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import type { ColorTheme } from "@/types/theme-props";
import type { SafeActionIntent } from "./model";

export interface SafeActionConfirmationDialogProps {
  intent: SafeActionIntent | null;
  pending: boolean;
  title: string;
  message: string;
  actionLabel: string;
  confirmLabel: string;
  cancelLabel: string;
  pendingLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
}

const stylesFor = (theme: ColorTheme) =>
  StyleSheet.create({
    overlay: {
      flex: 1,
      justifyContent: "center",
      padding: space.lg,
      backgroundColor: theme.overlay,
    },
    card: {
      gap: space.md,
      padding: space.lg,
      borderRadius: space.md,
      backgroundColor: theme.card,
    },
    title: {
      color: theme.foreground,
      fontSize: fontSizes.xl,
      fontWeight: fontWeights.medium,
    },
    message: { color: theme.mutedForeground, fontSize: fontSizes.md },
    binding: {
      gap: space.xs,
      padding: space.md,
      borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.border,
      borderRadius: space.sm,
      backgroundColor: theme.primaryMuted,
    },
    bindingText: {
      color: theme.foreground,
      fontSize: fontSizes.md,
      fontWeight: fontWeights.medium,
    },
    buttons: { flexDirection: "row", gap: space.sm },
    button: {
      flex: 1,
      minHeight: 44,
      alignItems: "center",
      justifyContent: "center",
      borderRadius: space.sm,
      borderWidth: 1,
      borderColor: theme.border,
    },
    confirm: { borderColor: theme.primary, backgroundColor: theme.primary },
    disabled: { opacity: 0.55 },
    cancelText: { color: theme.foreground, fontSize: fontSizes.lg },
    confirmText: { color: theme.white, fontSize: fontSizes.lg },
    pending: { flexDirection: "row", alignItems: "center", gap: space.sm },
  });

/**
 * Confirmation is visibly and structurally bound to both action and target.
 * All copy is supplied by the feature through translations.
 */
export function SafeActionConfirmationDialog({
  intent,
  pending,
  title,
  message,
  actionLabel,
  confirmLabel,
  cancelLabel,
  pendingLabel,
  onConfirm,
  onCancel,
}: SafeActionConfirmationDialogProps) {
  const theme = useTheme().colorTheme;
  const styles = stylesFor(theme);
  if (!intent) return null;

  return (
    <Modal
      visible
      transparent
      animationType="fade"
      onRequestClose={pending ? undefined : onCancel}
    >
      <View style={styles.overlay}>
        <View style={styles.card}>
          <Text style={styles.title}>{title}</Text>
          <Text style={styles.message}>{message}</Text>
          <View style={styles.binding}>
            <Text style={styles.bindingText}>{actionLabel}</Text>
            <Text style={styles.bindingText}>{intent.target.label}</Text>
          </View>
          <View style={styles.buttons}>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={cancelLabel}
              accessibilityState={{ disabled: pending }}
              disabled={pending}
              onPress={onCancel}
              style={[styles.button, pending && styles.disabled]}
            >
              <Text style={styles.cancelText}>{cancelLabel}</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={pending ? pendingLabel : confirmLabel}
              accessibilityState={{ busy: pending, disabled: pending }}
              disabled={pending}
              onPress={onConfirm}
              style={[
                styles.button,
                styles.confirm,
                pending && styles.disabled,
              ]}
            >
              {pending ? (
                <View style={styles.pending}>
                  <ActivityIndicator color={theme.white} />
                  <Text style={styles.confirmText}>{pendingLabel}</Text>
                </View>
              ) : (
                <Text style={styles.confirmText}>{confirmLabel}</Text>
              )}
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

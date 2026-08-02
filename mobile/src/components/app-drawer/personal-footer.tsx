import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { fontSizes, fontWeights, gutter, space } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAuth } from "@/features/auth/auth-provider";
import { LogoutController } from "@/features/auth/logout-controller";
import { identityDisplay } from "@/features/profile/current-user";
import { useCurrentUserState } from "@/features/profile/current-user-provider";
import type { ColorTheme } from "@/types/theme-props";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    footer: {
      borderTopWidth: StyleSheet.hairlineWidth,
      borderTopColor: theme.border,
      paddingTop: space.sm,
    },
    identityRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      paddingHorizontal: gutter,
      paddingVertical: space.md,
    },
    avatar: {
      width: 40,
      height: 40,
      borderRadius: 20,
      alignItems: "center",
      justifyContent: "center",
      backgroundColor: theme.primaryMuted,
    },
    avatarText: {
      color: theme.primary,
      fontSize: fontSizes.md,
      fontWeight: fontWeights.medium,
    },
    identityCopy: { flex: 1, minWidth: 0 },
    identityPrimary: {
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.medium,
      color: theme.foreground,
    },
    identitySecondary: {
      marginTop: space.xxs,
      fontSize: fontSizes.sm,
      color: theme.mutedForeground,
    },
    retryText: {
      marginTop: space.xxs,
      fontSize: fontSizes.sm,
      color: theme.primary,
      fontWeight: fontWeights.medium,
    },
    logoutRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      paddingHorizontal: gutter,
      paddingVertical: space.md,
      minHeight: 52,
    },
    logoutPressed: { opacity: 0.7 },
    logoutText: {
      flex: 1,
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.medium,
      color: theme.error,
    },
  });

/**
 * Fixed personal section pinned below the workspace list: avatar/initials, the
 * signed-in name/email (or an honest offline/unavailable/"Signed in" state with
 * retry), and a destructive Logout row. Logout requires a native confirmation
 * and reuses m2's local-first {@link useAuth().signOut}; it cannot be
 * double-submitted.
 */
export function PersonalFooter({
  onRequestClose,
}: {
  onRequestClose: () => void;
}) {
  const styles = useThemeStyle(getStyles);
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { signOut } = useAuth();
  const current = useCurrentUserState();
  const [pending, setPending] = useState(false);

  const mounted = useRef(true);
  useEffect(
    () => () => {
      mounted.current = false;
    },
    [],
  );

  const display = useMemo(
    () =>
      identityDisplay(
        current.status === "ready" ? current.user : null,
        t("drawer.signedIn"),
      ),
    [current, t],
  );

  const controller = useMemo(
    () =>
      new LogoutController({
        confirm: () =>
          new Promise<boolean>((resolve) => {
            Alert.alert(
              t("drawer.logoutConfirmTitle"),
              t("drawer.logoutConfirmMessage"),
              [
                {
                  text: t("common.cancel"),
                  style: "cancel",
                  onPress: () => resolve(false),
                },
                {
                  text: t("drawer.logout"),
                  style: "destructive",
                  onPress: () => resolve(true),
                },
              ],
              { cancelable: true, onDismiss: () => resolve(false) },
            );
          }),
        signOut: async () => {
          onRequestClose();
          await signOut();
        },
        onPending: (value) => {
          if (mounted.current) setPending(value);
        },
      }),
    [onRequestClose, signOut, t],
  );

  const onLogout = useCallback(() => {
    void controller.request();
  }, [controller]);

  const statusLine =
    current.status === "offline"
      ? t("drawer.identityOffline")
      : current.status === "unavailable"
        ? t("drawer.identityUnavailable")
        : current.status === "loading"
          ? t("drawer.identityLoading")
          : display.secondary;

  const canRetry =
    current.status === "offline" || current.status === "unavailable";

  return (
    <View style={styles.footer}>
      <View style={styles.identityRow}>
        <View style={styles.avatar}>
          {display.initials ? (
            <Text style={styles.avatarText}>{display.initials}</Text>
          ) : (
            <Ionicons name="person" size={20} color={theme.primary} />
          )}
        </View>
        <View
          style={styles.identityCopy}
          accessibilityRole="text"
          accessibilityLabel={t("drawer.identityAccessibility", {
            primary: display.primary,
            secondary: statusLine ?? "",
          })}
        >
          <Text style={styles.identityPrimary} numberOfLines={1}>
            {display.primary}
          </Text>
          {statusLine ? (
            <Text style={styles.identitySecondary} numberOfLines={1}>
              {statusLine}
            </Text>
          ) : null}
          {canRetry ? (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t("drawer.identityRetry")}
              hitSlop={8}
              onPress={() => current.retry()}
            >
              <Text style={styles.retryText}>{t("drawer.identityRetry")}</Text>
            </Pressable>
          ) : null}
        </View>
      </View>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={t("drawer.logout")}
        accessibilityState={{ disabled: pending, busy: pending }}
        disabled={pending}
        onPress={onLogout}
        style={({ pressed }) => [
          styles.logoutRow,
          pressed && styles.logoutPressed,
        ]}
      >
        <Ionicons name="log-out-outline" size={22} color={theme.error} />
        <Text style={styles.logoutText}>{t("drawer.logout")}</Text>
        {pending ? <ActivityIndicator color={theme.error} /> : null}
      </Pressable>
    </View>
  );
}

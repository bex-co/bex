import { useMemo, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { fontSizes, fontWeights, gutter, space } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { useTranslations } from "@/common/hooks/use-translations";
import { identityDisplay } from "@/features/profile/current-user";
import { useCurrentUserState } from "@/features/profile/current-user-provider";
import { SettingsModal } from "@/features/preferences/settings-modal";
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
      minHeight: 52,
    },
    identityPressed: { backgroundColor: theme.primaryMuted },
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
    // Retry is its own row (not nested in the settings pressable) so a tap
    // targets exactly one action; padded to line up under the identity text.
    retryRow: {
      paddingLeft: gutter + 40 + space.md,
      paddingRight: gutter,
      paddingBottom: space.md,
    },
    retryText: {
      fontSize: fontSizes.sm,
      color: theme.primary,
      fontWeight: fontWeights.medium,
    },
  });

/**
 * Fixed personal section pinned below the workspace list: the avatar/initials
 * and signed-in name/email (or an honest offline/unavailable/"Signed in" state
 * with retry). Tapping it opens the personal {@link SettingsModal} — color
 * scheme, language, and logout — so the drawer stays a menu and the settings
 * page owns the preferences and the destructive sign-out.
 */
export function PersonalFooter() {
  const styles = useThemeStyle(getStyles);
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const current = useCurrentUserState();
  const [settingsVisible, setSettingsVisible] = useState(false);

  const display = useMemo(
    () =>
      identityDisplay(
        current.status === "ready" ? current.user : null,
        t("drawer.signedIn"),
      ),
    [current, t],
  );

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
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={t("drawer.identityAccessibility", {
          primary: display.primary,
          secondary: statusLine ?? "",
        })}
        accessibilityHint={t("settings.open")}
        onPress={() => setSettingsVisible(true)}
        style={({ pressed }) => [
          styles.identityRow,
          pressed && styles.identityPressed,
        ]}
      >
        <View style={styles.avatar}>
          {display.initials ? (
            <Text style={styles.avatarText}>{display.initials}</Text>
          ) : (
            <Ionicons name="person" size={20} color={theme.primary} />
          )}
        </View>
        <View style={styles.identityCopy}>
          <Text style={styles.identityPrimary} numberOfLines={1}>
            {display.primary}
          </Text>
          {statusLine ? (
            <Text style={styles.identitySecondary} numberOfLines={1}>
              {statusLine}
            </Text>
          ) : null}
        </View>
        <Ionicons
          name="chevron-forward"
          size={20}
          color={theme.mutedForeground}
        />
      </Pressable>

      {canRetry ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={t("drawer.identityRetry")}
          hitSlop={8}
          onPress={() => current.retry()}
          style={styles.retryRow}
        >
          <Text style={styles.retryText}>{t("drawer.identityRetry")}</Text>
        </Pressable>
      ) : null}

      <SettingsModal
        visible={settingsVisible}
        onClose={() => setSettingsVisible(false)}
      />
    </View>
  );
}

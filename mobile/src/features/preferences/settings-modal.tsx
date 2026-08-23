import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ActivityIndicator,
  Alert,
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { SafeAreaProvider, SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import Constants from "expo-constants";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { useTranslations } from "@/common/hooks/use-translations";
import { useThemeMode } from "@/common/providers/theme-provider";
import type {
  SupportedLanguage,
  ThemeMode,
} from "@/common/preferences/preferences";
import { useAuth } from "@/features/auth/auth-provider";
import { LogoutController } from "@/features/auth/logout-controller";
import { identityDisplay } from "@/features/profile/current-user";
import { useCurrentUserState } from "@/features/profile/current-user-provider";
import type { ColorTheme } from "@/types/theme-props";

const THEME_OPTIONS: {
  value: ThemeMode;
  labelKey: string;
  icon: keyof typeof Ionicons.glyphMap;
}[] = [
  { value: "system", labelKey: "settings.themeSystem", icon: "contrast" },
  { value: "light", labelKey: "settings.themeLight", icon: "sunny" },
  { value: "dark", labelKey: "settings.themeDark", icon: "moon" },
];

// Language labels stay in their own script — a language menu you cannot read is
// useless, so these are intentionally not run through translation.
const LANGUAGE_OPTIONS: { value: SupportedLanguage; label: string }[] = [
  { value: "en", label: "English" },
  { value: "zh", label: "中文" },
];

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    safe: { flex: 1, backgroundColor: theme.background },
    header: {
      flexDirection: "row",
      alignItems: "center",
      paddingHorizontal: gutter,
      paddingVertical: space.sm,
    },
    title: {
      flex: 1,
      fontSize: fontSizes.xxl,
      fontWeight: "700",
      color: theme.foreground,
    },
    close: {
      width: 40,
      minHeight: 44,
      alignItems: "flex-end",
      justifyContent: "center",
    },
    content: { paddingBottom: space.xxl },
    account: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      paddingHorizontal: gutter,
      paddingVertical: space.lg,
    },
    avatar: {
      width: 52,
      height: 52,
      borderRadius: 26,
      alignItems: "center",
      justifyContent: "center",
      backgroundColor: theme.primaryMuted,
    },
    avatarText: {
      color: theme.primary,
      fontSize: fontSizes.xl,
      fontWeight: fontWeights.medium,
    },
    accountCopy: { flex: 1, minWidth: 0 },
    accountPrimary: {
      fontSize: fontSizes.xl,
      fontWeight: fontWeights.medium,
      color: theme.foreground,
    },
    accountSecondary: {
      marginTop: space.xxs,
      fontSize: fontSizes.sm,
      color: theme.mutedForeground,
    },
    sectionLabel: {
      fontSize: fontSizes.sm,
      fontWeight: fontWeights.medium,
      color: theme.mutedForeground,
      paddingHorizontal: gutter,
      paddingTop: space.lg,
      paddingBottom: space.sm,
    },
    group: {
      backgroundColor: theme.card,
      borderTopWidth: StyleSheet.hairlineWidth,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderColor: theme.border,
    },
    optionRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      paddingHorizontal: gutter,
      paddingVertical: space.md,
      minHeight: 52,
    },
    optionPressed: { backgroundColor: theme.primaryMuted },
    optionLabel: {
      flex: 1,
      fontSize: fontSizes.lg,
      color: theme.foreground,
    },
    logout: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      marginTop: space.xxl,
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
    version: {
      marginTop: space.lg,
      textAlign: "center",
      fontSize: fontSizes.sm,
      color: theme.mutedForeground,
    },
  });

/**
 * Full-screen personal settings sheet reached from the drawer avatar: the
 * signed-in identity, color-scheme and language preferences (persisted on this
 * device), and the destructive logout. Logout requires a native confirmation
 * and reuses m2's local-first {@link useAuth().signOut}; it cannot be
 * double-submitted. This is not a route — the supervision-scope policy keeps
 * new routes out of `app/`, so it presents as a React Native Modal.
 */
export function SettingsModal({
  visible,
  onClose,
}: {
  visible: boolean;
  onClose: () => void;
}) {
  const styles = useThemeStyle(getStyles);
  const theme = useTheme().colorTheme;
  const { t, language, setLanguage } = useTranslations();
  const { mode, setMode } = useThemeMode();
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
          onClose();
          await signOut();
        },
        onPending: (value) => {
          if (mounted.current) setPending(value);
        },
      }),
    [onClose, signOut, t],
  );

  const onLogout = useCallback(() => {
    void controller.request();
  }, [controller]);

  const version = Constants.expoConfig?.version ?? "—";

  return (
    <Modal
      visible={visible}
      animationType="slide"
      onRequestClose={onClose}
      presentationStyle="fullScreen"
    >
      {/* A fullScreen Modal renders in its own native hierarchy that the app's
          SafeAreaProvider does not reach, so its insets would resolve to 0 and
          the header would sit under the status bar / Dynamic Island. A provider
          scoped to the modal measures the real insets. */}
      <SafeAreaProvider>
        <SafeAreaView style={styles.safe} edges={["top", "bottom"]}>
          <View style={styles.header}>
            <Text style={styles.title}>{t("settings.title")}</Text>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t("settings.close")}
              hitSlop={12}
              onPress={onClose}
              style={styles.close}
            >
              <Ionicons name="close" size={26} color={theme.foreground} />
            </Pressable>
          </View>

          <ScrollView contentContainerStyle={styles.content}>
            <View style={styles.account}>
              <View style={styles.avatar}>
                {display.initials ? (
                  <Text style={styles.avatarText}>{display.initials}</Text>
                ) : (
                  <Ionicons name="person" size={26} color={theme.primary} />
                )}
              </View>
              <View style={styles.accountCopy}>
                <Text style={styles.accountPrimary} numberOfLines={1}>
                  {display.primary}
                </Text>
                {display.secondary ? (
                  <Text style={styles.accountSecondary} numberOfLines={1}>
                    {display.secondary}
                  </Text>
                ) : null}
              </View>
            </View>

            <Text style={styles.sectionLabel}>{t("settings.colorScheme")}</Text>
            <View style={styles.group}>
              {THEME_OPTIONS.map((option) => {
                const selected = option.value === mode;
                return (
                  <Pressable
                    key={option.value}
                    accessibilityRole="radio"
                    accessibilityState={{ selected }}
                    accessibilityLabel={t(option.labelKey)}
                    onPress={() => setMode(option.value)}
                    style={({ pressed }) => [
                      styles.optionRow,
                      pressed && styles.optionPressed,
                    ]}
                  >
                    <Ionicons
                      name={option.icon}
                      size={22}
                      color={theme.mutedForeground}
                    />
                    <Text style={styles.optionLabel}>{t(option.labelKey)}</Text>
                    {selected ? (
                      <Ionicons
                        name="checkmark"
                        size={22}
                        color={theme.primary}
                      />
                    ) : null}
                  </Pressable>
                );
              })}
            </View>

            <Text style={styles.sectionLabel}>{t("settings.language")}</Text>
            <View style={styles.group}>
              {LANGUAGE_OPTIONS.map((option) => {
                const selected = option.value === language;
                return (
                  <Pressable
                    key={option.value}
                    accessibilityRole="radio"
                    accessibilityState={{ selected }}
                    accessibilityLabel={option.label}
                    onPress={() => setLanguage(option.value)}
                    style={({ pressed }) => [
                      styles.optionRow,
                      pressed && styles.optionPressed,
                    ]}
                  >
                    <Text style={styles.optionLabel}>{option.label}</Text>
                    {selected ? (
                      <Ionicons
                        name="checkmark"
                        size={22}
                        color={theme.primary}
                      />
                    ) : null}
                  </Pressable>
                );
              })}
            </View>

            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t("drawer.logout")}
              accessibilityState={{ disabled: pending, busy: pending }}
              disabled={pending}
              onPress={onLogout}
              style={({ pressed }) => [
                styles.logout,
                pressed && styles.logoutPressed,
              ]}
            >
              <Ionicons name="log-out-outline" size={22} color={theme.error} />
              <Text style={styles.logoutText}>{t("drawer.logout")}</Text>
              {pending ? <ActivityIndicator color={theme.error} /> : null}
            </Pressable>

            <Text style={styles.version}>
              {t("settings.version", { version })}
            </Text>
          </ScrollView>
        </SafeAreaView>
      </SafeAreaProvider>
    </Modal>
  );
}

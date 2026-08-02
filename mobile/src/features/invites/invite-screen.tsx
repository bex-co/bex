import { useEffect, useRef, useState } from "react";
import { useLocalSearchParams, useRouter } from "expo-router";
import {
  ActivityIndicator,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Button } from "@/components/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { useAuth } from "@/features/auth/auth-provider";
import { useInvite } from "./invite-provider";
import type { InviteFlowState } from "./invite-controller";
import { bootstrapInviteLink } from "./invite-link-bootstrap";

export function InviteScreen() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const { width } = useWindowDimensions();
  const router = useRouter();
  const params = useLocalSearchParams<{ invite?: string | string[] }>();
  const { state: auth, signIn } = useAuth();
  const invite = useInvite();
  const captured = useRef(false);
  const [signingIn, setSigningIn] = useState(false);
  const [signInFailed, setSignInFailed] = useState(false);

  useEffect(() => {
    if (captured.current || params.invite === undefined) return;
    captured.current = true;
    const value = params.invite;
    void bootstrapInviteLink(
      value,
      () => router.replace("/invite"),
      invite.capture,
    ).catch(() => undefined);
  }, [invite, params.invite, router]);

  const content = inviteContent(invite.state, auth.status, signInFailed, t);
  const busy =
    invite.state.status === "loading" ||
    invite.state.status === "accepting" ||
    auth.status === "loading" ||
    signingIn;
  const action =
    invite.state.status === "ready" && auth.status === "signedIn"
      ? () => void invite.accept()
      : invite.state.status === "retryable" && auth.status === "signedIn"
        ? () => void invite.accept()
        : (invite.state.status === "accepted" ||
              invite.state.status === "terminal" ||
              invite.state.status === "empty") &&
            auth.status === "signedIn"
          ? () => router.replace("/(app)")
          : (invite.state.status === "ready" ||
                invite.state.status === "retryable") &&
              auth.status !== "loading"
            ? () => {
                setSigningIn(true);
                setSignInFailed(false);
                signIn()
                  .catch(() => setSignInFailed(true))
                  .finally(() => setSigningIn(false));
              }
            : undefined;

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <View
        accessibilityRole="summary"
        style={[
          styles.content,
          { width: Math.min(Math.max(0, width - gutter * 2), 560) },
        ]}
      >
        {busy ? <ActivityIndicator color={theme.primary} size="large" /> : null}
        <Text style={[styles.title, { color: theme.foreground }]}>
          {content.title}
        </Text>
        <Text style={[styles.body, { color: theme.mutedForeground }]}>
          {content.body}
        </Text>
        {invite.state.status === "accepted" && invite.state.refreshFailed ? (
          <Text
            accessibilityRole="alert"
            style={[styles.note, { color: theme.warning }]}
          >
            {t("invite.refreshFailed")}
          </Text>
        ) : null}
        {content.action && action ? (
          <Button
            style={styles.action}
            onPress={action}
            disabled={busy}
            loading={busy}
            accessibilityLabel={content.action}
          >
            {content.action}
          </Button>
        ) : null}
      </View>
    </SafeAreaView>
  );
}

function inviteContent(
  state: InviteFlowState,
  authStatus: "loading" | "signedOut" | "expired" | "signedIn",
  signInFailed: boolean,
  t: (key: string, options?: Record<string, unknown>) => string,
): { title: string; body: string; action?: string } {
  if (state.status === "loading" || authStatus === "loading") {
    return {
      title: t("invite.title"),
      body: t("invite.loading"),
    };
  }
  if (state.status === "accepted") {
    return {
      title: t("invite.acceptedTitle"),
      body: t("invite.acceptedBody", {
        workspace: state.workspace.name ?? state.workspace.id,
      }),
      action: t("invite.openWorkspace"),
    };
  }
  if (state.status === "terminal") {
    return {
      title: t("invite.failedTitle"),
      body: t(`invite.failures.${state.failure}`),
      action: authStatus === "signedIn" ? t("common.backToStatus") : undefined,
    };
  }
  if (state.status === "empty") {
    return {
      title: t("invite.failedTitle"),
      body: t("invite.empty"),
      action: authStatus === "signedIn" ? t("common.backToStatus") : undefined,
    };
  }
  if (authStatus !== "signedIn") {
    return {
      title: t("invite.title"),
      body: t(signInFailed ? "invite.signInError" : "invite.signInBody"),
      action: t("invite.signIn"),
    };
  }
  if (state.status === "accepting") {
    return { title: t("invite.title"), body: t("invite.accepting") };
  }
  if (state.status === "retryable") {
    return {
      title: t("invite.retryTitle"),
      body: t(`invite.retryable.${state.failure}`),
      action: t("invite.retry"),
    };
  }
  return {
    title: t("invite.title"),
    body: t("invite.readyBody"),
    action: t("invite.accept"),
  };
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: {
    flex: 1,
    alignSelf: "center",
    justifyContent: "center",
    gap: space.md,
  },
  title: {
    fontSize: fontSizes.xxl,
    fontWeight: fontWeights.medium,
    textAlign: "center",
  },
  body: { fontSize: fontSizes.md, lineHeight: 24, textAlign: "center" },
  note: { fontSize: fontSizes.sm, lineHeight: 20, textAlign: "center" },
  action: { alignSelf: "stretch", marginTop: space.sm },
});

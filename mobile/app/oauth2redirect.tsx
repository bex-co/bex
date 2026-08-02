import { useEffect, useMemo, useRef, useState } from "react";
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { AuthStateScreen } from "@/features/auth/auth-screen";
import { useAuth } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";

const CALLBACK_KEYS = [
  "code",
  "state",
  "error",
  "error_description",
  "error_uri",
] as const;

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

export default function OAuthRedirect() {
  const params =
    useLocalSearchParams<
      Partial<Record<(typeof CALLBACK_KEYS)[number], string | string[]>>
    >();
  const router = useRouter();
  const { state, completeSignIn } = useAuth();
  const started = useRef(false);
  const [failed, setFailed] = useState(false);
  const redirectUrl = useMemo(() => {
    const query = new URLSearchParams();
    for (const key of CALLBACK_KEYS) {
      const value = first(params[key]);
      if (value !== undefined) query.set(key, value);
    }
    const encoded = query.toString();
    return `${mobileConfig.oauthRedirectUri}${encoded ? `?${encoded}` : ""}`;
  }, [params]);

  useEffect(() => {
    // On a cold callback, AuthProvider first checks SecureStore for an existing
    // session. Wait for that read so it cannot overwrite the new session after
    // the authorization-code exchange finishes.
    if (state.status === "loading" || state.status === "signedIn") return;
    if (started.current) return;
    started.current = true;
    completeSignIn(redirectUrl).catch(() => setFailed(true));
  }, [completeSignIn, redirectUrl, state.status]);

  if (state.status === "signedIn") return <Redirect href="/(app)" />;
  if (failed) {
    return (
      <AuthStateScreen
        titleKey="auth.signInTitle"
        bodyKey="auth.signInError"
        actionKey="auth.retry"
        onAction={() => router.replace("/sign-in")}
      />
    );
  }
  return (
    <AuthStateScreen
      titleKey="auth.callbackTitle"
      bodyKey="auth.callbackBody"
      busy
    />
  );
}

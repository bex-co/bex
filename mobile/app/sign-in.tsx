import { useState } from "react";
import { Redirect } from "expo-router";
import { AuthStateScreen } from "@/features/auth/auth-screen";
import { useAuth } from "@/features/auth/auth-provider";

export default function SignInScreen() {
  const { state, signIn, retryRestore } = useAuth();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  if (state.status === "signedIn") return <Redirect href="/(app)" />;
  if (state.status === "loading") {
    return (
      <AuthStateScreen
        titleKey="auth.loadingTitle"
        bodyKey="auth.loadingBody"
        busy
      />
    );
  }
  if (state.status === "expired") {
    return (
      <AuthStateScreen
        titleKey="auth.expiredTitle"
        bodyKey="auth.expiredBody"
        actionKey="auth.retry"
        busy={busy}
        onAction={() => {
          setBusy(true);
          retryRestore()
            .catch(() => undefined)
            .finally(() => setBusy(false));
        }}
      />
    );
  }
  return (
    <AuthStateScreen
      titleKey="auth.signInTitle"
      bodyKey={failed ? "auth.signInError" : "auth.signInBody"}
      actionKey="auth.signInAction"
      busy={busy}
      onAction={() => {
        setBusy(true);
        setFailed(false);
        signIn()
          .catch(() => setFailed(true))
          .finally(() => setBusy(false));
      }}
    />
  );
}

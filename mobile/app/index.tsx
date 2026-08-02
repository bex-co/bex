import { Redirect } from "expo-router";
import { AuthStateScreen } from "@/features/auth/auth-screen";
import { useAuth } from "@/features/auth/auth-provider";

export default function Index() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <AuthStateScreen
        titleKey="auth.loadingTitle"
        bodyKey="auth.loadingBody"
        busy
      />
    );
  }
  return (
    <Redirect href={state.status === "signedIn" ? "/(app)" : "/sign-in"} />
  );
}

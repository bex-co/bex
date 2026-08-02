import { router } from "expo-router";
import { AuthStateScreen } from "@/features/auth/auth-screen";

export function InvalidDeepLinkScreen() {
  return (
    <AuthStateScreen
      titleKey="deepLink.invalidTitle"
      bodyKey="deepLink.invalidBody"
      actionKey="common.backToStatus"
      onAction={() => router.replace("/")}
    />
  );
}

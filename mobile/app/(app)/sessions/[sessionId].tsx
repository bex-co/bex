import { useLocalSearchParams } from "expo-router";
import { ShellScreen } from "@/components/shell-screen";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validAgentSessionDeepLink } from "@/features/navigation/deep-link";

export default function AgentSessionDeepLinkScreen() {
  const { sessionId } = useLocalSearchParams<{
    sessionId?: string | string[];
  }>();
  if (!validAgentSessionDeepLink(sessionId)) return <InvalidDeepLinkScreen />;
  return (
    <ShellScreen
      titleKey="deepLink.sessionTitle"
      bodyKey="deepLink.sessionBody"
      icon="sparkles-outline"
    />
  );
}

import { useLocalSearchParams } from "expo-router";
import { InvalidDeepLinkScreen } from "@/features/navigation/invalid-deep-link-screen";
import { validAgentSessionDeepLink } from "@/features/navigation/deep-link";
import { SessionDetailScreen } from "@/features/agent-sessions/detail/session-detail-screen";

export default function AgentSessionDeepLinkScreen() {
  const { sessionId } = useLocalSearchParams<{
    sessionId?: string | string[];
  }>();
  if (!validAgentSessionDeepLink(sessionId)) return <InvalidDeepLinkScreen />;
  return <SessionDetailScreen sessionId={sessionId} />;
}

import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";
import type { UIMessage } from "ai";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, space, useTheme } from "@/common/theme";
import { useConversationSeed } from "../attach/use-conversation-seed";
import { useAgentSessionChat } from "../attach/use-agent-session-chat";
import { AgentConversationList } from "./conversation-list";
import { SessionInteractions } from "../steering/session-interactions";

export function SessionConversation({
  sessionId,
  liveEnabled,
  header,
  footer,
  refreshing,
  onRefresh,
}: {
  sessionId: string;
  liveEnabled: boolean;
  header?: ReactNode;
  footer?: ReactNode;
  refreshing?: boolean;
  onRefresh?: () => void;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const seed = useConversationSeed(sessionId);
  if (seed.status === "loading") {
    return (
      <AgentConversationList
        messages={[]}
        status="submitted"
        header={header}
        footer={footer}
        refreshing={refreshing}
        onRefresh={onRefresh}
        notice={
          <Text style={[styles.notice, { color: theme.mutedForeground }]}>
            {t("agentSessions.conversation.loading")}
          </Text>
        }
      />
    );
  }
  return liveEnabled ? (
    <LiveConversation
      sessionId={sessionId}
      initialMessages={seed.messages}
      initialCursor={seed.cursor}
      historyDegraded={Boolean(seed.error)}
      header={header}
      footer={footer}
      refreshing={refreshing}
      onRefresh={onRefresh}
    />
  ) : (
    <AgentConversationList
      messages={seed.messages}
      status="ready"
      header={header}
      footer={footer}
      refreshing={refreshing}
      onRefresh={onRefresh}
      notice={seed.error ? <UnavailableNotice /> : null}
    />
  );
}

function LiveConversation({
  sessionId,
  initialMessages,
  initialCursor,
  historyDegraded,
  header,
  footer,
  refreshing,
  onRefresh,
}: {
  sessionId: string;
  initialMessages: UIMessage[];
  initialCursor: number;
  historyDegraded: boolean;
  header?: ReactNode;
  footer?: ReactNode;
  refreshing?: boolean;
  onRefresh?: () => void;
}) {
  const chat = useAgentSessionChat({
    sessionId,
    initialMessages,
    initialCursor,
  });
  return (
    <AgentConversationList
      messages={chat.messages}
      status={chat.status}
      header={header}
      footer={footer}
      refreshing={refreshing}
      onRefresh={onRefresh}
      notice={historyDegraded || chat.error ? <UnavailableNotice /> : null}
      interaction={
        <SessionInteractions
          sessionId={sessionId}
          chat={{
            status: chat.status,
            error: chat.error,
            available: chat.available,
            recoveryPhase: chat.recovery.phase,
            sendMessage: chat.sendMessage,
          }}
        />
      }
    />
  );
}

function UnavailableNotice() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View
      accessibilityRole="alert"
      style={[styles.alert, { borderColor: theme.warning }]}
    >
      <Text style={[styles.notice, { color: theme.mutedForeground }]}>
        {t("agentSessions.conversation.unavailable")}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  notice: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
  alert: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
    padding: space.sm,
  },
});

import { useCallback, useEffect, useState, type ComponentType } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import type { MintedTicket } from "@/features/agent-sessions/lib/transport";
import type {
  ConversationChatHandle,
  SessionConversationImplProps,
} from "@/features/agent-sessions/components/session-conversation-impl";

export type { ConversationChatHandle };

// The client-only entry point for the conversation column. The AI SDK
// (`ai` + `@ai-sdk/react`) and the ticket-minting transport live entirely in
// `session-conversation-impl`, which this wrapper loads via a dynamic import in
// an effect — the web-shell-terminal precedent. Nothing here pulls the
// transport into the SSR bundle: on the server (and the first client render)
// this renders only a lightweight placeholder, so `yarn build`'s SSR pass never
// evaluates `useChat` or constructs a transport.

export interface SessionConversationProps {
  sessionId: string;
  isTerminal: boolean;
  /**
   * Lifts the live-steering handle up to the detail page (t004). Passed straight
   * through to the client-only impl so the steering composer can send a live
   * turn through this column's own `useChat` instance.
   */
  onChatStateChange?: (handle: ConversationChatHandle | null) => void;
}

export function SessionConversation({
  sessionId,
  isTerminal,
  onChatStateChange,
}: SessionConversationProps) {
  const { t } = useTranslations();
  const { attach } = useAgentSessionMutations();
  const [Impl, setImpl] =
    useState<ComponentType<SessionConversationImplProps> | null>(null);

  useEffect(() => {
    let live = true;
    void import("@/features/agent-sessions/components/session-conversation-impl").then(
      (module) => {
        if (live) setImpl(() => module.SessionConversationImpl);
      },
    );
    return () => {
      live = false;
    };
  }, []);

  // Each (re)connection mints a fresh attach ticket through t001's attach verb —
  // the 90s TTL makes per-connect minting mandatory (transport wires this into
  // both the send headers and `prepareReconnectToStreamRequest`).
  const mintTicket = useCallback(async (): Promise<MintedTicket> => {
    const minted = await attach(sessionId);
    if (!minted.ticket) {
      throw new Error("attachAgentSession returned no ticket");
    }
    return { ticket: minted.ticket, url: minted.url };
  }, [attach, sessionId]);

  if (!Impl) {
    return (
      <div className="text-muted-foreground p-4 text-sm">
        {t("agentSessions.conversationLoading")}
      </div>
    );
  }

  return (
    <Impl
      sessionId={sessionId}
      isTerminal={isTerminal}
      mintTicket={mintTicket}
      onChatStateChange={onChatStateChange}
    />
  );
}

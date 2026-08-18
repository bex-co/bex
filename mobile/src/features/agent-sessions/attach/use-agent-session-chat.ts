import { useCallback, useEffect, useMemo } from "react";
import { useMutation } from "@apollo/client/react";
import { useChat } from "@ai-sdk/react";
import { fetch as expoFetch } from "expo/fetch";
import type { UIMessage } from "ai";
import { MobileAttachAgentSessionDocument } from "@/generated-graphql";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import {
  createAgentSessionTransport,
  type AgentTicketAction,
  type MintedAgentTicket,
} from "./transport";
import { isTerminalAttachFailure } from "./recovery-errors";

export type UseAgentSessionChatOptions = {
  sessionId: string;
  initialMessages: UIMessage[];
  initialCursor: number;
};

/**
 * Native AI SDK 6 chat instance. The component using this hook mounts only
 * after the durable seed is ready; `resume` then opens one cursor-based GET.
 */
export function useAgentSessionChat({
  sessionId,
  initialMessages,
  initialCursor,
}: UseAgentSessionChatOptions) {
  const [attach] = useMutation(MobileAttachAgentSessionDocument);
  const mintTicket = useCallback(
    async (action: AgentTicketAction): Promise<MintedAgentTicket> => {
      const result = await attach({ variables: { id: sessionId, action } });
      const minted = result.data?.attachAgentSession;
      if (!minted?.ticket) {
        throw new Error("attachAgentSession returned no ticket");
      }
      return {
        ticket: minted.ticket,
        url: minted.url,
        expiresAt: minted.expiresAt,
      };
    },
    [attach, sessionId],
  );
  const transport = useMemo(
    () =>
      createAgentSessionTransport({
        sessionId,
        initialCursor,
        mintTicket,
        fetch: expoFetch as unknown as typeof globalThis.fetch,
      }),
    [initialCursor, mintTicket, sessionId],
  );
  const chat = useChat({
    id: sessionId,
    transport,
    messages: initialMessages,
    resume: false,
    // Native lists should not rerender on every token while a large tool or
    // terminal part streams. The stream remains lossless; only UI publication
    // is coalesced.
    experimental_throttle: 50,
  });

  const environment = useRecoveryEnvironment();
  const recovery = useRecovery({
    attempt: async () => {
      chat.clearError();
      await chat.resumeStream();
    },
    maxAttempts: 3,
    isRetryable: (error) => !isTerminalAttachFailure(error),
  });

  useEffect(() => {
    void recovery.reconnectStream();
    // Initial attach is one recovery request; callbacks remain stable for the
    // lifetime of this chat instance.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!recoveryAvailable(environment)) void chat.stop();
  }, [chat.stop, environment]);

  useEffect(() => {
    const stop = chat.stop;
    return () => {
      void stop();
    };
  }, [chat.stop]);

  return { ...chat, recovery, available: recoveryAvailable(environment) };
}

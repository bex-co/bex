// Edits to the cached queue of waiting messages (ADR047 D12), so a send, an
// edit, a move or a removal shows at once and the next read only confirms it.
// The queue lives in its own query (`AgentSessionQueue`), keyed by session; a
// session whose queue was never read has nothing cached to edit.

import type { ApolloCache } from "@apollo/client";
import {
  AgentSessionQueueDocument,
  type AgentSessionQueuedMessageFieldsFragment,
} from "@/graphql/definitions";

type WireMessage = AgentSessionQueuedMessageFieldsFragment;

/** Positions are 1-based places in the queue, so every edit renumbers them. */
function renumbered(messages: readonly WireMessage[]): WireMessage[] {
  return messages.map((message, index) => ({
    ...message,
    position: index + 1,
  }));
}

/** The queue with `message` last; a retried send of the same message is not doubled. */
export function withQueuedMessage(
  messages: readonly WireMessage[],
  message: WireMessage,
): WireMessage[] {
  return renumbered([
    ...messages.filter((other) => other.id !== message.id),
    message,
  ]);
}

/** The queue with the message of the same id replaced (an edit's answer). */
export function withEditedMessage(
  messages: readonly WireMessage[],
  message: WireMessage,
): WireMessage[] {
  return renumbered(
    messages.map((other) => (other.id === message.id ? message : other)),
  );
}

/** The queue without the message. */
export function withoutQueuedMessage(
  messages: readonly WireMessage[],
  messageId: string,
): WireMessage[] {
  return renumbered(messages.filter((other) => other.id !== messageId));
}

/**
 * Rewrites one session's cached queue and its pending count. Does nothing when
 * the queue was never read: the next read fills it.
 */
export function patchCachedQueue(
  cache: Pick<ApolloCache, "updateQuery">,
  sessionId: string,
  update: (messages: readonly WireMessage[]) => WireMessage[],
): void {
  cache.updateQuery(
    { query: AgentSessionQueueDocument, variables: { id: sessionId } },
    (data) => {
      const session = data?.agentSession;
      if (!session) return undefined;
      const queuedMessages = renumbered(update(session.queuedMessages ?? []));
      return {
        agentSession: {
          ...session,
          queuedMessages,
          pendingMessages: queuedMessages.length,
        },
      };
    },
  );
}

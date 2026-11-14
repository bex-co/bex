import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionQueueDocument } from "@/graphql/definitions";
import { useRefetchOnChange } from "@/common/hooks/use-refetch-on-change";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { toQueuedMessageViews } from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionView,
  QueuedMessageView,
} from "@/features/agent-sessions/types";
import { AGENT_SESSION_ACTIVE_POLL_MS } from "@/features/agent-sessions/hooks/use-agent-sessions";

const NO_MESSAGES: QueuedMessageView[] = [];

/** A session that can hold messages: not archived, not canceled (which clears them). */
export function canHoldQueue(
  session: Pick<AgentSessionView, "phase" | "isArchived">,
): boolean {
  return (
    !session.isArchived &&
    session.phase !== "canceling" &&
    session.phase !== "canceled"
  );
}

/**
 * The messages waiting behind a session's turn, next to run first (ADR047
 * D12). The queue is its own small read of `agentSession(id)`, polled at the
 * session's own active cadence while a turn runs or a message waits, and read
 * again whenever the session's phase or turn count changes: the moment a turn
 * settles and the dispatcher starts the next message, the list drops it. The
 * queue mutations edit this query's cache directly, so a send, move, edit or
 * removal shows before the next read. An idle session with nothing queued is
 * read once, not polled.
 */
export function useAgentSessionQueue(
  session: AgentSessionView,
): QueuedMessageView[] {
  const queueable = canHoldQueue(session);
  const { data, refetch, startPolling, stopPolling } = useQuery(
    AgentSessionQueueDocument,
    {
      variables: { id: session.id },
      skip: !queueable,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const wire = data?.agentSession?.queuedMessages;
  const messages = useMemo(() => toQueuedMessageViews(wire), [wire]);

  const busy = !session.isTerminal || messages.length > 0;
  useEffect(() => {
    if (queueable && busy) startPolling(AGENT_SESSION_ACTIVE_POLL_MS);
    else stopPolling();
  }, [queueable, busy, startPolling, stopPolling]);

  useRefetchOnChange(`${session.phase}:${session.turns}`, refetch, queueable);

  return queueable ? messages : NO_MESSAGES;
}

import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionTreeDocument } from "@/graphql/definitions";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { toAgentSessionViews } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import {
  AGENT_SESSION_ACTIVE_POLL_MS,
  AGENT_SESSION_IDLE_POLL_MS,
} from "@/features/agent-sessions/hooks/use-agent-sessions";

export interface UseAgentSessionTreeResult {
  /** The tree the session belongs to: root first, then by depth and creation. */
  sessions: AgentSessionView[];
  /** True until the first answer lands (a poll never re-announces it). */
  loading: boolean;
}

/**
 * Reads `agentSessionTree(id)` (ADR102 D8) for the detail page: the child cards
 * and the Agents panel. Polls like `useAgentSession` — fast while any session
 * in the tree is still working, the baseline once all have settled.
 */
export function useAgentSessionTree(id: string): UseAgentSessionTreeResult {
  const { data, loading, startPolling } = useQuery(AgentSessionTreeDocument, {
    variables: { id },
    skip: !id,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
    skipPollAttempt: skipPollWhenHidden,
  });

  const sessions = useMemo(
    () => toAgentSessionViews(data?.agentSessionTree?.sessions),
    [data],
  );

  const active = sessions.length === 0 || sessions.some((s) => !s.isTerminal);
  useEffect(() => {
    if (!id) return;
    startPolling(
      active ? AGENT_SESSION_ACTIVE_POLL_MS : AGENT_SESSION_IDLE_POLL_MS,
    );
  }, [active, id, startPolling]);

  return { sessions, loading: loading && data === undefined };
}

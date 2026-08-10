import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionDocument } from "@/graphql/definitions";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { toAgentSessionView } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import {
  AGENT_SESSION_ACTIVE_POLL_MS,
  AGENT_SESSION_IDLE_POLL_MS,
} from "@/features/agent-sessions/hooks/use-agent-sessions";

export interface UseAgentSessionResult {
  session: AgentSessionView | null;
  loading: boolean;
  error: Error | undefined;
  /** Re-read the session now (used after a lifecycle mutation converges). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `agentSession(id)` query for the detail page (ADR047 D9).
 * Metadata only — phase/PR/turns; the conversation column is the m43
 * stream, never polled here. Polls at 5s while the session is non-terminal (or
 * not yet loaded) so the header converges on its own, then drops to 30s once
 * terminal so an out-of-band change (a teammate cancel) still surfaces.
 */
export function useAgentSession(id: string): UseAgentSessionResult {
  const { data, loading, error, refetch, startPolling } = useQuery(
    AgentSessionDocument,
    {
      variables: { id },
      skip: !id,
      fetchPolicy: "cache-first",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const session = useMemo(
    () => (data?.agentSession ? toAgentSessionView(data.agentSession) : null),
    [data],
  );

  // Poll fast until settled: not yet loaded (treated as active), or still
  // non-terminal; drop to the baseline once terminal.
  const active = session ? !session.isTerminal : true;
  useEffect(() => {
    if (!id) return;
    startPolling(
      active ? AGENT_SESSION_ACTIVE_POLL_MS : AGENT_SESSION_IDLE_POLL_MS,
    );
  }, [active, id, startPolling]);

  return { session, loading, error, refetch };
}

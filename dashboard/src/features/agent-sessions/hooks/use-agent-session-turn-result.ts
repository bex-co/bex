import { useEffect, useRef } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionTurnResultsDocument } from "@/graphql/definitions";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { AGENT_SESSION_ACTIVE_POLL_MS } from "@/features/agent-sessions/hooks/use-agent-sessions";

/** The latest turn's structured-output state (ADR094 D5). */
export interface AgentSessionTurnResult {
  turn: number;
  /** JSON text of the last valid result the agent saved, final or not. */
  result: string;
  /** The agent declared the result final (it becomes `outcome.result`). */
  final: boolean;
}

/**
 * The latest turn's in-flight structured result, for the "draft" block. Only a
 * non-final value matters here (the final one rides `outcome.result`), so the
 * read polls while the session is live and settles to one read once terminal.
 * One more read lands when the session settles, so the draft shown is the one
 * the turn ended with. Parts are never fetched.
 */
export function useAgentSessionTurnResult(
  id: string,
  live: boolean,
): AgentSessionTurnResult | null {
  const { data, refetch } = useQuery(AgentSessionTurnResultsDocument, {
    variables: { id },
    skip: !id,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: live ? AGENT_SESSION_ACTIVE_POLL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });
  const wasLive = useRef(live);
  useEffect(() => {
    const settled = wasLive.current && !live;
    wasLive.current = live;
    if (id && settled) void refetch().catch(() => undefined);
  }, [id, live, refetch]);
  const latest = data?.agentSessionTranscript.turns.at(-1);
  if (!latest || latest.result == null) return null;
  return {
    turn: latest.turn,
    result: latest.result,
    final: latest.resultFinal,
  };
}

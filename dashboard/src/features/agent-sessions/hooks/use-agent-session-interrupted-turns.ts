import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionTurnInterruptsDocument } from "@/graphql/definitions";
import { useRefetchOnChange } from "@/common/hooks/use-refetch-on-change";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { AGENT_SESSION_ACTIVE_POLL_MS } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { toInterruptedTurns } from "@/features/agent-sessions/lib/mapper";
import type { InterruptedTurns } from "@/features/agent-sessions/types";

const NO_TURNS: InterruptedTurns = new Map();

/**
 * The turns a person stopped with Stop turn (ADR047 D12), so the transcript
 * marks them "interrupted" rather than leave them looking cut off or failed,
 * and every turn whose undelivered work was kept or lost (w10/m491: it
 * failed, idled out or was followed by a message), each with what became of
 * that work (w22/m413): kept until a time, being saved, continued, or lost. An interrupted turn settles as a completed
 * one, so the stream carries no trace of the stop: only the turn's own
 * `interrupted` flag says so. It is read when the page opens and again
 * whenever `settleKey` changes (the page passes its attach signal: a turn
 * settling, or the next one starting, changes it), by which point the flag is
 * written: the Completer records it before the session leaves `running`. The
 * disk is saved after that, for up to a few minutes, so a turn whose save is
 * still running is read again every few seconds until it is kept or lost.
 */
export function useAgentSessionInterruptedTurns(
  sessionId: string,
  settleKey: string | undefined,
): InterruptedTurns {
  const { data, refetch, startPolling, stopPolling } = useQuery(
    AgentSessionTurnInterruptsDocument,
    {
      variables: { id: sessionId },
      skip: !sessionId,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  useRefetchOnChange(settleKey, refetch, Boolean(sessionId));

  const turns = data?.agentSessionTranscript.turns;
  const interrupted = useMemo(() => {
    const read = toInterruptedTurns(turns);
    return read.size > 0 ? read : NO_TURNS;
  }, [turns]);

  const saving = useMemo(
    () =>
      [...interrupted.values()].some(
        (turn) => turn.retention.state === "saving",
      ),
    [interrupted],
  );
  useEffect(() => {
    if (saving) startPolling(AGENT_SESSION_ACTIVE_POLL_MS);
    else stopPolling();
    return stopPolling;
  }, [saving, startPolling, stopPolling]);

  return interrupted;
}

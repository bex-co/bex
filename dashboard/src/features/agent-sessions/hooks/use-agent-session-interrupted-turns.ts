import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionTurnInterruptsDocument } from "@/graphql/definitions";
import { useRefetchOnChange } from "@/common/hooks/use-refetch-on-change";

const NO_TURNS: ReadonlySet<number> = new Set();

/**
 * The turns a person stopped with Stop turn (ADR047 D12), so the transcript
 * marks them "interrupted" rather than leave them looking cut off or failed.
 * An interrupted turn settles as a completed one, so the stream carries no
 * trace of the stop: only the turn's own `interrupted` flag says so. It is
 * read when the page opens and again whenever `settleKey` changes (the page
 * passes its attach signal: a turn settling, or the next one starting, changes
 * it), by which point the flag is written: the Completer records it before the
 * session leaves `running`.
 */
export function useAgentSessionInterruptedTurns(
  sessionId: string,
  settleKey: string | undefined,
): ReadonlySet<number> {
  const { data, refetch } = useQuery(AgentSessionTurnInterruptsDocument, {
    variables: { id: sessionId },
    skip: !sessionId,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  useRefetchOnChange(settleKey, refetch, Boolean(sessionId));

  const turns = data?.agentSessionTranscript.turns;
  return useMemo(() => {
    const interrupted = (turns ?? [])
      .filter((turn) => turn.interrupted)
      .map((turn) => turn.turn);
    return interrupted.length > 0 ? new Set(interrupted) : NO_TURNS;
  }, [turns]);
}

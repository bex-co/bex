import { useQuery } from "@apollo/client/react";
import { AgentSessionTurnSkillsDocument } from "@/graphql/definitions";
import type { AgentSessionSkill } from "@/graphql/schema-types";

/**
 * The workspace skills one turn invoked (ADR099 D4), for the transcript's
 * pills. Every turn of a session reads the same small query (no parts), so
 * Apollo shares one request; a newly mounted turn refreshes it so the turn a
 * steer just added finds its skills.
 */
export function useAgentSessionTurnSkills(
  sessionId: string,
  turn: number | null,
): Pick<AgentSessionSkill, "name" | "arguments">[] {
  const { data } = useQuery(AgentSessionTurnSkillsDocument, {
    variables: { id: sessionId },
    skip: !sessionId || turn == null,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  if (turn == null) return [];
  return (
    data?.agentSessionTranscript.turns.find((t) => t.turn === turn)?.skills ??
    []
  );
}

import { useEffect, useRef } from "react";
import { useQuery } from "@apollo/client/react";
import {
  AgentSessionModelCostDocument,
  type AgentSessionModelCostQuery,
} from "@/graphql/definitions";

export type AgentSessionModelCost =
  AgentSessionModelCostQuery["agentSessionModelCost"];

/** What the session cost read depends on: its turns and whether it is live. */
interface CostTrigger {
  turns: number;
  live: boolean;
}

/**
 * A session's bex-provided model cost, in total and per turn (ADR091 D6).
 * Read once on mount, then re-read only when the session moves on: a new turn
 * starts (its predecessor's requests are in), or the session finishes (the
 * last turn's are). No timer polling: cost only changes with turns.
 */
export function useAgentSessionModelCost(
  id: string,
  turns: number,
  live: boolean,
): { cost: AgentSessionModelCost | null; loading: boolean } {
  const { data, loading, refetch } = useQuery(AgentSessionModelCostDocument, {
    variables: { id },
    skip: !id,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const previous = useRef<CostTrigger>({ turns, live });
  useEffect(() => {
    const before = previous.current;
    previous.current = { turns, live };
    // The mount's own query already read the current state.
    const finished = before.live && !live;
    if (id && (turns !== before.turns || finished)) {
      void refetch().catch(() => undefined);
    }
  }, [id, turns, live, refetch]);
  const cost = data?.agentSessionModelCost ?? null;
  return { cost, loading: loading && cost == null };
}

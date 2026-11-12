import { useQuery } from "@apollo/client/react";
import { AgentSessionDocument } from "@/graphql/definitions";
import type { AgentSessionTurnOrigin } from "@/graphql/schema-types";

export type TurnOrigin = Pick<
  AgentSessionTurnOrigin,
  "kind" | "author" | "url"
> &
  Partial<Pick<AgentSessionTurnOrigin, "turn">>;

const NO_ORIGINS: TurnOrigin[] = [];

/**
 * What PR feedback started one steering turn (w9/m344): a PR comment, a review
 * or a failed CI run, several when one steer delivered a queue of them. Turn 1
 * has one only when a `/bex` comment started the session (w9/m348). Reads
 * the detail page's own `agentSession(id)` query, which the page already
 * holds and polls, so a turn adds no request and a just-delivered steer's
 * origin appears on the page's next poll.
 */
export function useAgentSessionTurnOrigins(
  sessionId: string,
  turn: number | null,
): TurnOrigin[] {
  const { data } = useQuery(AgentSessionDocument, {
    variables: { id: sessionId },
    skip: !sessionId || turn == null,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  if (turn == null) return NO_ORIGINS;
  const origins = data?.agentSession?.turnOrigins?.filter(
    (origin) => origin.turn === turn,
  );
  return origins?.length ? origins : NO_ORIGINS;
}

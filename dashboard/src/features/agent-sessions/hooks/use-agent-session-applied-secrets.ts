import { useQuery } from "@apollo/client/react";
import { AgentSessionDocument } from "@/graphql/definitions";
import type { AgentAppliedSecret } from "@/graphql/schema-types";

export type AppliedSecret = Pick<AgentAppliedSecret, "name" | "source">;

const NONE: AppliedSecret[] = [];

/**
 * The secrets the session's latest turn received, each with its source
 * (w9/m385, ADR098 D9): inline, explicit (picked), repo or workspace. Names
 * only. Reads the detail page's own `agentSession(id)` query, which the page
 * already holds and polls, so it adds no request and a new turn's list
 * appears on the page's next poll.
 */
export function useAgentSessionAppliedSecrets(
  sessionId: string,
): AppliedSecret[] {
  const { data } = useQuery(AgentSessionDocument, {
    variables: { id: sessionId },
    skip: !sessionId,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const applied = data?.agentSession?.appliedSecrets;
  return applied?.length ? applied : NONE;
}

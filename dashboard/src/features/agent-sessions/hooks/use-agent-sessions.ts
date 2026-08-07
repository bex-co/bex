import { useEffect, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { AgentSessionsDocument } from "@/graphql/definitions";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { toAgentSessionViews } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** Fast cadence while any session is still converging (a live transcript moves). */
export const AGENT_SESSION_ACTIVE_POLL_MS = 5_000;
/** Baseline cadence once every session has settled (out-of-band changes only). */
export const AGENT_SESSION_IDLE_POLL_MS = 30_000;

export interface UseAgentSessionsOptions {
  /**
   * Poll for out-of-band changes (default `true`). Pass `false` on a secondary
   * consumer mounted alongside a polling one: every `useQuery` gets its own
   * timer, and two timers reschedule off their own responses, so they drift
   * apart into separate round trips instead of deduplicating.
   */
  poll?: boolean;
}

export interface UseAgentSessionsResult {
  sessions: AgentSessionView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the list now (callers refresh after a create/cancel). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `agentSessions(ownerId)` query and maps each Render-shaped
 * `AgentSession` onto a normalized view (ADR047 D9). Metadata only — the live
 * conversation is NOT polled; it rides the same-origin stream endpoint (t002).
 *
 * Polls at 5s while any session is non-terminal so a running transcript's
 * metadata (phase/turns/PR) converges on its own, then falls back to 30s once
 * every row is terminal. Scoped to the switcher's selected workspace (mirrors
 * `useDatabases`/`useServices`); `skip` until the workspace resolves, which is
 * also the `!authenticated` guard — the workspace context only resolves for a
 * signed-in caller.
 */
export function useAgentSessions({
  poll = true,
}: UseAgentSessionsOptions = {}): UseAgentSessionsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch, startPolling } = useQuery(
    AgentSessionsDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      skip: !resolved,
      fetchPolicy: "cache-first",
      errorPolicy: "all",
      pollInterval: poll ? AGENT_SESSION_IDLE_POLL_MS : 0,
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const sessions = useMemo(
    () => toAgentSessionViews(data?.agentSessions),
    [data],
  );

  // Switch cadence on whether anything is still converging: any non-terminal
  // row polls fast; an all-terminal (or not-yet-loaded → treated as active)
  // list falls back to the baseline.
  const anyActive =
    sessions.length === 0 || sessions.some((s) => !s.isTerminal);
  useEffect(() => {
    if (!resolved || !poll) return;
    startPolling(
      anyActive ? AGENT_SESSION_ACTIVE_POLL_MS : AGENT_SESSION_IDLE_POLL_MS,
    );
  }, [anyActive, resolved, poll, startPolling]);

  return {
    sessions,
    loading: !resolved || loading,
    error,
    refetch,
  };
}

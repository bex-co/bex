import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  /**
   * Archive membership (ADR065 D3): omitted/"false" ⇒ the unarchived working
   * set (the backend default), "true" ⇒ archived only, "all" ⇒ both.
   */
  archived?: "false" | "true" | "all";
  /** Phase filter (repeatable); omitted ⇒ every phase. */
  phases?: string[];
  /** Exact owner/repository filter; omitted ⇒ every repo. */
  repo?: string;
  /** Page size; omitted ⇒ the backend default (50, max 200). */
  limit?: number;
}

export interface UseAgentSessionsResult {
  sessions: AgentSessionView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the list now (callers refresh after a create/cancel). */
  refetch: () => Promise<unknown>;
  /**
   * Fetch the next keyset page (cursor = the current last session id) and
   * concatenate it (ADR065 D3). Only meaningful on a non-polling consumer —
   * a poll rewrites the base cache entry back to page one.
   */
  loadMore: () => Promise<void>;
  loadingMore: boolean;
  /** False once a page came back shorter than the page size (the end). */
  hasMore: boolean;
}

/** The backend's default page size (agentsessions defaultListLimit). */
export const AGENT_SESSION_PAGE_SIZE = 50;

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
  archived,
  phases,
  repo,
  limit,
}: UseAgentSessionsOptions = {}): UseAgentSessionsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch, startPolling, fetchMore } = useQuery(
    AgentSessionsDocument,
    {
      variables: {
        ownerId: currentWorkspaceId,
        archived: archived ?? null,
        phases: phases?.length ? phases : null,
        repo: repo || null,
        limit: limit ?? null,
      },
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

  // Keyset pagination (ADR065 D3): cursor = the last item's id; a page shorter
  // than the page size is the end. Filter changes reset the exhaustion flag.
  const pageSize = limit ?? AGENT_SESSION_PAGE_SIZE;
  const requestKey = JSON.stringify([
    currentWorkspaceId,
    archived ?? "",
    phases ?? [],
    repo ?? "",
    limit ?? 0,
  ]);
  const [loadingMore, setLoadingMore] = useState(false);
  const [exhausted, setExhausted] = useState(false);
  useEffect(() => {
    setExhausted(false);
  }, [requestKey]);

  const sessionsRef = useRef(sessions);
  sessionsRef.current = sessions;
  const loadMore = useCallback(async () => {
    const last = sessionsRef.current.at(-1);
    if (!last) return;
    setLoadingMore(true);
    try {
      const result = await fetchMore({
        variables: { cursor: last.id },
        updateQuery(previous, { fetchMoreResult }) {
          const seen = new Set(previous.agentSessions.map((s) => s.id));
          return {
            ...previous,
            agentSessions: [
              ...previous.agentSessions,
              ...fetchMoreResult.agentSessions.filter((s) => !seen.has(s.id)),
            ],
          };
        },
      });
      if ((result.data?.agentSessions.length ?? 0) < pageSize) {
        setExhausted(true);
      }
    } finally {
      setLoadingMore(false);
    }
  }, [fetchMore, pageSize]);

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
    loadMore,
    loadingMore,
    hasMore: !exhausted && sessions.length >= pageSize,
  };
}

import { useCallback, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  AgentAutomationsDocument,
  type AgentAutomationFieldsFragment,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export type AutomationView = AgentAutomationFieldsFragment;

/** Rows per page; the backend caps a page at 200. */
export const AUTOMATION_PAGE_SIZE = 50;

export interface UseAgentAutomationsResult {
  automations: AutomationView[];
  /** Loading with no data yet: the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  loadMore: () => Promise<void>;
  loadingMore: boolean;
  hasMore: boolean;
}

/** The workspace's automations, newest first; `mine` keeps the caller's own. */
export function useAgentAutomations(mine: boolean): UseAgentAutomationsResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch, fetchMore } = useQuery(
    AgentAutomationsDocument,
    {
      variables: {
        ownerId: currentWorkspaceId ?? "",
        mine,
        limit: AUTOMATION_PAGE_SIZE,
      },
      skip: currentWorkspaceId == null,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
    },
  );
  const [loadingMore, setLoadingMore] = useState(false);
  const list = data?.agentAutomations ?? null;
  const nextCursor = list?.nextCursor ?? null;

  const loadMore = useCallback(async () => {
    if (!nextCursor) return;
    setLoadingMore(true);
    try {
      await fetchMore({
        variables: { cursor: nextCursor },
        updateQuery(previous, { fetchMoreResult }) {
          const seen = new Set(
            previous.agentAutomations.automations.map((a) => a.id),
          );
          return {
            agentAutomations: {
              ...fetchMoreResult.agentAutomations,
              automations: [
                ...previous.agentAutomations.automations,
                ...fetchMoreResult.agentAutomations.automations.filter(
                  (a) => !seen.has(a.id),
                ),
              ],
            },
          };
        },
      });
    } finally {
      setLoadingMore(false);
    }
  }, [fetchMore, nextCursor]);

  return {
    automations: list?.automations ?? [],
    loading: currentWorkspaceId == null || (loading && list == null),
    error,
    refetch,
    loadMore,
    loadingMore,
    hasMore: nextCursor != null,
  };
}

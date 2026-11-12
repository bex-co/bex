import { useCallback, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  AgentAutomationRunsDocument,
  type AgentAutomationRunsQuery,
} from "@/graphql/definitions";
import { isAutomationId } from "@/features/agent-automations/lib/automation-id";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export type AutomationRunRow =
  AgentAutomationRunsQuery["agentAutomationRuns"]["runs"][number];

export const RUN_PAGE_SIZE = 20;

export interface UseAutomationRunsResult {
  runs: AutomationRunRow[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  loadMore: () => Promise<void>;
  loadingMore: boolean;
  hasMore: boolean;
}

/** An automation's run history, newest first, a cursor page at a time. */
export function useAutomationRuns(id: string): UseAutomationRunsResult {
  const { currentWorkspaceId } = useWorkspace();
  const byId = isAutomationId(id);
  const waiting = !byId && currentWorkspaceId == null;
  const { data, loading, error, refetch, fetchMore } = useQuery(
    AgentAutomationRunsDocument,
    {
      variables: byId
        ? { id, limit: RUN_PAGE_SIZE }
        : { ownerId: currentWorkspaceId ?? "", id, limit: RUN_PAGE_SIZE },
      skip: waiting,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
    },
  );
  const [loadingMore, setLoadingMore] = useState(false);
  const list = data?.agentAutomationRuns ?? null;
  const nextCursor = list?.nextCursor ?? null;

  const loadMore = useCallback(async () => {
    if (!nextCursor) return;
    setLoadingMore(true);
    try {
      await fetchMore({
        variables: { cursor: nextCursor },
        updateQuery(previous, { fetchMoreResult }) {
          return {
            agentAutomationRuns: {
              ...fetchMoreResult.agentAutomationRuns,
              runs: [
                ...previous.agentAutomationRuns.runs,
                ...fetchMoreResult.agentAutomationRuns.runs,
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
    runs: list?.runs ?? [],
    loading: waiting || (loading && list == null),
    error,
    refetch,
    loadMore,
    loadingMore,
    hasMore: nextCursor != null,
  };
}

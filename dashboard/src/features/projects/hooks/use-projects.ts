import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { ProjectsDocument } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface ProjectView {
  id: string;
  name: string;
  ownerId: string;
  serviceIds: string[];
  databaseIds: string[];
  keyValueIds: string[];
}

export interface UseProjectsOptions {
  /**
   * Poll for out-of-band changes (default `true`). Pass `false` on a secondary
   * consumer mounted alongside a polling one: every `useQuery` gets its own
   * timer, and two timers reschedule off their own responses, so they drift
   * apart into separate round trips instead of deduplicating.
   */
  poll?: boolean;
}

export interface UseProjectsResult {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (fire-and-forget; callers refresh after a create/rename/move). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads the projects for the current workspace. Returns an empty list when
 * the workspace hasn't resolved yet or when the store isn't configured.
 */
export function useProjects({
  poll = true,
}: UseProjectsOptions = {}): UseProjectsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(ProjectsDocument, {
    variables: { ownerId: currentWorkspaceId! },
    skip: !resolved,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
    pollInterval: poll ? RESOURCE_POLL_INTERVAL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });

  const projects = useMemo((): ProjectView[] => {
    if (!data?.projects) return [];
    return data.projects
      .filter((p): p is NonNullable<typeof p> => p != null)
      .map((p) => ({
        id: p.id ?? "",
        name: p.name ?? "",
        ownerId: p.ownerId ?? "",
        serviceIds: (p.serviceIds ?? []).filter((s): s is string => s != null),
        databaseIds: (p.databaseIds ?? []).filter(
          (s): s is string => s != null,
        ),
        keyValueIds: (p.keyValueIds ?? []).filter(
          (s): s is string => s != null,
        ),
      }));
  }, [data]);

  return { projects, loading: !resolved || loading, error, refetch };
}

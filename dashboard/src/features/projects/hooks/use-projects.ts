import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
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
export function useProjects(): UseProjectsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(ProjectsDocument, {
    variables: { ownerId: currentWorkspaceId! },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
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

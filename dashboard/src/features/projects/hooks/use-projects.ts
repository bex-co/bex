import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ProjectsDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface ProjectView {
  id: string;
  name: string;
  ownerId: string;
  serviceIds: string[];
}

export interface UseProjectsResult {
  projects: ProjectView[];
  loading: boolean;
  error: Error | undefined;
}

/**
 * Reads the projects for the current workspace. Returns an empty list when
 * the workspace hasn't resolved yet or when the store isn't configured.
 */
export function useProjects(): UseProjectsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error } = useQuery(ProjectsDocument, {
    variables: { ownerId: currentWorkspaceId! },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
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
      }));
  }, [data]);

  return { projects, loading: !resolved || loading, error };
}

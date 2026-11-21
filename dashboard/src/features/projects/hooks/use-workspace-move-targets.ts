import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { EnvGroupScopeIndexDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { mapProjects } from "@/features/projects/hooks/use-projects";
import { mapEnvironments } from "@/features/environments/hooks/use-environments";
import { environmentScopeName } from "@/features/env-groups/lib/scope";
import type { BulkMoveTarget } from "@/features/projects/hooks/use-bulk-move";

/**
 * Every environment of the workspace as a bulk Move target, labelled
 * `project / environment` because environment names repeat across projects
 * (w4/215). The Overview's ungrouped list moves into any of them, as Render's
 * Move picker offers every project's environments (w4/m550).
 *
 * A watched query, not a one-shot read: a Move full-replaces the target's
 * membership from these ids, and the `setEnvironment*` result updates the
 * normalized Environment in place, so a second Move never drops the first.
 */
export function useWorkspaceMoveTargets(): BulkMoveTarget[] {
  const { currentWorkspaceId } = useWorkspace();
  const { data } = useQuery(EnvGroupScopeIndexDocument, {
    variables: { ownerId: currentWorkspaceId ?? "" },
    skip: !currentWorkspaceId,
    fetchPolicy: "cache-and-network",
  });
  return useMemo(() => {
    const projects = mapProjects(data?.projects, currentWorkspaceId ?? "");
    return mapEnvironments(data?.workspaceEnvironments).map((environment) => ({
      environment,
      label: environmentScopeName(environment, projects),
    }));
  }, [data, currentWorkspaceId]);
}

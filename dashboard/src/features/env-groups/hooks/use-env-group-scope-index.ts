import { useEffect, useMemo, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { EnvGroupScopeIndexDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  mapProjects,
  type ProjectView,
} from "@/features/projects/hooks/use-projects";
import {
  mapEnvironments,
  type EnvironmentView,
} from "@/features/environments/hooks/use-environments";

const EMPTY_PROJECTS: ProjectView[] = [];
const EMPTY_ENVIRONMENTS: EnvironmentView[] = [];

/** The selected-workspace scope index used by list, detail, and link filtering. */
export function useEnvGroupScopeIndex() {
  const { currentWorkspaceId } = useWorkspace();
  return useWorkspaceEnvironmentIndex(currentWorkspaceId);
}

/**
 * Loads one authorized workspace's Projects and Environments. The request
 * generation is tied to ownerId, so late responses can never repopulate a
 * dialog or page after a workspace switch.
 */
export function useWorkspaceEnvironmentIndex(ownerId: string | null) {
  const client = useApolloClient();
  const [snapshot, setSnapshot] = useState<{
    ownerId: string | null;
    projects: ProjectView[];
    environments: EnvironmentView[];
    error?: Error;
  }>({ ownerId: null, projects: [], environments: [] });

  useEffect(() => {
    let active = true;
    if (!ownerId) return () => void (active = false);
    void client
      .query({
        query: EnvGroupScopeIndexDocument,
        variables: { ownerId },
        fetchPolicy: "cache-first",
        errorPolicy: "none",
      })
      .then((result) => {
        const loadedProjects = mapProjects(result.data?.projects, ownerId);
        const loadedEnvironments = mapEnvironments(
          result.data?.workspaceEnvironments,
        );
        if (active) {
          setSnapshot({
            ownerId,
            projects: loadedProjects,
            environments: loadedEnvironments,
          });
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setSnapshot({
            ownerId,
            projects: [],
            environments: [],
            error:
              cause instanceof Error
                ? cause
                : new Error("environment index failed"),
          });
        }
      });
    return () => {
      active = false;
    };
  }, [client, ownerId]);

  const current = snapshot.ownerId === ownerId;
  const projects = current ? snapshot.projects : EMPTY_PROJECTS;
  const environments = current ? snapshot.environments : EMPTY_ENVIRONMENTS;
  const error = current ? snapshot.error : undefined;
  const loading = ownerId != null && !current;

  const byId = useMemo(
    () =>
      new Map(environments.map((environment) => [environment.id, environment])),
    [environments],
  );
  const serviceEnvironmentById = useMemo(() => {
    const index = new Map<string, string>();
    for (const environment of environments) {
      for (const serviceId of environment.serviceIds) {
        index.set(serviceId, environment.id);
      }
    }
    return index;
  }, [environments]);

  return {
    projects,
    environments,
    byId,
    serviceEnvironmentById,
    loading,
    error,
  };
}

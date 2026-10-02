import { useCallback, useEffect, useMemo, useState } from "react";
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
  const [attempt, setAttempt] = useState(0);
  const [snapshot, setSnapshot] = useState<{
    ownerId: string | null;
    attempt: number;
    ready: boolean;
    projects: ProjectView[];
    environments: EnvironmentView[];
    error?: Error;
  }>({
    ownerId: null,
    attempt: -1,
    ready: false,
    projects: [],
    environments: [],
  });
  const retry = useCallback(() => setAttempt((current) => current + 1), []);

  useEffect(() => {
    let active = true;
    if (!ownerId) return () => void (active = false);
    void client
      .query({
        query: EnvGroupScopeIndexDocument,
        variables: { ownerId },
        fetchPolicy: attempt === 0 ? "cache-first" : "network-only",
        errorPolicy: "none",
      })
      .then((result) => {
        if (
          !Array.isArray(result.data?.projects) ||
          !Array.isArray(result.data?.workspaceEnvironments)
        ) {
          throw new Error("environment index is incomplete");
        }
        const loadedProjects = mapProjects(result.data?.projects, ownerId);
        const loadedEnvironments = mapEnvironments(
          result.data?.workspaceEnvironments,
        );
        if (active) {
          setSnapshot({
            ownerId,
            attempt,
            ready: true,
            projects: loadedProjects,
            environments: loadedEnvironments,
          });
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setSnapshot((previous) => ({
            ownerId,
            attempt,
            ready: previous.ownerId === ownerId && previous.ready,
            projects: previous.ownerId === ownerId ? previous.projects : [],
            environments:
              previous.ownerId === ownerId ? previous.environments : [],
            error:
              cause instanceof Error
                ? cause
                : new Error("environment index failed"),
          }));
        }
      });
    return () => {
      active = false;
    };
  }, [client, ownerId, attempt]);

  const current = ownerId != null && snapshot.ownerId === ownerId;
  const projects = current ? snapshot.projects : EMPTY_PROJECTS;
  const environments = current ? snapshot.environments : EMPTY_ENVIRONMENTS;
  const settled = current && snapshot.attempt === attempt;
  const error = settled ? snapshot.error : undefined;
  const loading = ownerId != null && !settled;
  const ready = current && snapshot.ready;

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
    ownerId,
    projects,
    environments,
    byId,
    serviceEnvironmentById,
    ready,
    loading,
    error,
    retry,
  };
}

import { useCallback, useMemo } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentRepoEnvironmentsDocument,
  DeleteAgentRepoEnvironmentDocument,
  SetAgentRepoEnvironmentDocument,
} from "@/graphql/definitions";
import type {
  RepoEnvironmentInput,
  RepoEnvironmentView,
} from "@/features/agent-environment/lib/repo-environments";
import { isActiveBuild } from "@/features/agent-environment/lib/repo-environment-builds";
import { useBuildPolling } from "@/features/agent-environment/hooks/use-build-polling";

export interface UseAgentRepoEnvironmentsResult {
  /** The workspace's repository environments, in the server's order. */
  environments: RepoEnvironmentView[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  /** Creates or replaces a repository's whole environment (ADR109 D7). */
  set: (repo: string, environment: RepoEnvironmentInput) => Promise<void>;
  remove: (repo: string) => Promise<void>;
  saving: boolean;
}

/**
 * Lists and writes the workspace's repository environments (ADR109), each
 * with its newest build (ADR111). While any of those builds is queued or
 * building, the list re-reads itself so the row's status follows the build.
 */
export function useAgentRepoEnvironments(
  ownerId: string | null,
): UseAgentRepoEnvironmentsResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    AgentRepoEnvironmentsDocument,
    {
      variables: { ownerId: ownerId ?? "" },
      skip: ownerId == null,
      fetchPolicy: "cache-and-network",
    },
  );
  const [setMutation, { loading: setting }] = useMutation(
    SetAgentRepoEnvironmentDocument,
  );
  const [deleteMutation, { loading: deleting }] = useMutation(
    DeleteAgentRepoEnvironmentDocument,
  );

  const set = useCallback(
    async (repo: string, environment: RepoEnvironmentInput) => {
      if (ownerId == null) throw new Error("no workspace selected");
      await setMutation({
        variables: {
          ownerId,
          repo,
          setup: environment.setup,
          checks: environment.checks,
        },
        refetchQueries: [AgentRepoEnvironmentsDocument],
        awaitRefetchQueries: true,
      });
    },
    [ownerId, setMutation],
  );

  const remove = useCallback(
    async (repo: string) => {
      if (ownerId == null) throw new Error("no workspace selected");
      await deleteMutation({
        variables: { ownerId, repo },
        refetchQueries: [AgentRepoEnvironmentsDocument],
        awaitRefetchQueries: true,
      });
    },
    [ownerId, deleteMutation],
  );

  const list = data?.agentRepoEnvironments ?? null;
  const environments = useMemo<RepoEnvironmentView[]>(
    () =>
      list?.environments.map((env) => ({
        repo: env.repo,
        version: env.version,
        setup: env.setup.map(({ name, run }) => ({ name, run })),
        checks: env.checks.map(({ name, run }) => ({ name, run })),
        latestBuild: env.latestBuild ?? null,
      })) ?? [],
    [list],
  );
  useBuildPolling(
    environments.some((env) => isActiveBuild(env.latestBuild)),
    startPolling,
    stopPolling,
  );
  return {
    environments,
    loading: ownerId == null || (loading && list == null),
    error,
    refetch,
    set,
    remove,
    saving: setting || deleting,
  };
}

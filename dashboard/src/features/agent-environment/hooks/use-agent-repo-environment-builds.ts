import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  AgentRepoEnvironmentBuildDocument,
  AgentRepoEnvironmentBuildsDocument,
  AgentRepoEnvironmentsDocument,
  BuildAgentRepoEnvironmentDocument,
  type AgentRepoEnvironmentBuildQuery,
} from "@/graphql/definitions";
import {
  isActiveBuild,
  type RepoEnvironmentBuildView,
} from "@/features/agent-environment/lib/repo-environment-builds";
import { useBuildPolling } from "@/features/agent-environment/hooks/use-build-polling";

/** One build read on its own: the list's fields plus its log. */
export type RepoEnvironmentBuildDetail =
  AgentRepoEnvironmentBuildQuery["agentRepoEnvironmentBuild"];

export interface UseAgentRepoEnvironmentBuildsResult {
  /** The repository's last 20 builds, newest first (ADR111 D8). */
  builds: RepoEnvironmentBuildView[];
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  retry: () => void;
  /** Build now (admins only, ADR111 D3): resolves to the queued build. */
  buildNow: () => Promise<RepoEnvironmentBuildView>;
  /** A Build now is in flight. */
  starting: boolean;
}

/**
 * A repository's environment builds, newest first, and Build now (ADR111
 * D10). While one of them is queued or building, the list re-reads itself.
 */
export function useAgentRepoEnvironmentBuilds(
  ownerId: string,
  repo: string,
): UseAgentRepoEnvironmentBuildsResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    AgentRepoEnvironmentBuildsDocument,
    {
      variables: { ownerId, repo },
      fetchPolicy: "cache-and-network",
    },
  );
  const [mutate, { loading: starting }] = useMutation(
    BuildAgentRepoEnvironmentDocument,
  );
  const list = data?.agentRepoEnvironmentBuilds ?? null;
  const builds = list?.builds ?? [];
  useBuildPolling(builds.some(isActiveBuild), startPolling, stopPolling);

  const buildNow = useCallback(async () => {
    const res = await mutate({
      variables: { ownerId, repo },
      // The row's newest build and this history both gain the queued build.
      refetchQueries: [
        AgentRepoEnvironmentsDocument,
        AgentRepoEnvironmentBuildsDocument,
      ],
      awaitRefetchQueries: true,
    });
    const build = res.data?.buildAgentRepoEnvironment;
    if (!build) throw new Error("Build now returned no build");
    return build;
  }, [mutate, ownerId, repo]);

  return {
    builds,
    loading: loading && list == null,
    error,
    retry: () => void refetch(),
    buildNow,
    starting,
  };
}

export interface UseAgentRepoEnvironmentBuildResult {
  build: RepoEnvironmentBuildDetail | null;
  loading: boolean;
  error: Error | undefined;
  retry: () => void;
}

/**
 * One build with its log (ADR111 D10). A queued or building build is
 * re-read until it settles, then once more for its final log.
 */
export function useAgentRepoEnvironmentBuild(
  ownerId: string,
  id: string | null,
): UseAgentRepoEnvironmentBuildResult {
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    AgentRepoEnvironmentBuildDocument,
    {
      variables: { ownerId, id: id ?? "" },
      skip: id == null,
      fetchPolicy: "cache-first",
    },
  );
  const detail = data?.agentRepoEnvironmentBuild ?? null;
  const build = detail != null && detail.id === id ? detail : null;
  useBuildPolling(isActiveBuild(build), startPolling, stopPolling, () => {
    if (id != null) void refetch();
  });
  return {
    build,
    loading: id != null && loading && build == null,
    error,
    retry: () => void refetch(),
  };
}

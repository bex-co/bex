import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  BuildSourceBuildDocument,
  BuildSourceBuildsDocument,
} from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { isActiveBuildStatus } from "@/features/build-sources/lib/build-status";
import {
  mapBuildSourceBuild,
  type BuildSourceBuildView,
} from "@/features/build-sources/types";

const ACTIVE_BUILD_POLL_MS = 5000;

export interface UseBuildSourceBuildsResult {
  builds: BuildSourceBuildView[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error: Error | undefined;
  loadMore: () => Promise<void>;
  refetch: () => Promise<BuildSourceBuildView[]>;
}

/**
 * Lists builds produced by a Build Source (newest first) with cursor-based
 * "Load more" pagination and bounded live polling while any build is active.
 */
export function useBuildSourceBuilds(
  buildSourceId: string | null,
  enabled = true,
  limit = 20,
  options: { poll?: boolean } = {},
): UseBuildSourceBuildsResult {
  const poll = options.poll ?? false;
  const [loadingMore, setLoadingMore] = useState(false);
  const [exhausted, setExhausted] = useState<{
    id: string | null;
    done: boolean;
  }>({ id: buildSourceId, done: false });

  if (exhausted.id !== buildSourceId) {
    setExhausted({ id: buildSourceId, done: false });
  }

  const [activePoll, setActivePoll] = useState(false);
  const { data, loading, error, fetchMore, refetch } = useQuery(
    BuildSourceBuildsDocument,
    {
      variables: { buildSourceId: buildSourceId ?? "", limit },
      skip: !enabled || !buildSourceId,
      fetchPolicy: "cache-and-network",
      nextFetchPolicy: "cache-first",
      errorPolicy: "all",
      pollInterval: poll && activePoll ? ACTIVE_BUILD_POLL_MS : 0,
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const builds = useMemo(
    () =>
      (data?.builds ?? []).flatMap((raw) => {
        const mapped = mapBuildSourceBuild(raw, buildSourceId ?? "");
        return mapped ? [mapped] : [];
      }),
    [data, buildSourceId],
  );

  const hasActiveBuild = builds.some((b) => isActiveBuildStatus(b.status));
  if (poll && activePoll !== hasActiveBuild) {
    setActivePoll(hasActiveBuild);
  }

  const lastCursor = builds.at(-1)?.cursor ?? null;
  const hasMore =
    !exhausted.done &&
    builds.length > 0 &&
    builds.length % limit === 0 &&
    lastCursor !== null;

  const loadMore = useCallback(async () => {
    if (!buildSourceId || !lastCursor || loadingMore || !hasMore) return;
    setLoadingMore(true);
    try {
      const res = await fetchMore({
        variables: { buildSourceId, cursor: lastCursor, limit },
        updateQuery: (prev, { fetchMoreResult }) => {
          const prevList = prev.builds ?? [];
          const nextList = fetchMoreResult.builds ?? [];
          const seen = new Set(prevList.map((b) => b?.id));
          const merged = [
            ...prevList,
            ...nextList.filter((b) => !seen.has(b?.id)),
          ];
          return { ...prev, builds: merged };
        },
      });
      const returned = res.data?.builds ?? [];
      if (returned.length < limit) {
        setExhausted({ id: buildSourceId, done: true });
      }
    } finally {
      setLoadingMore(false);
    }
  }, [buildSourceId, fetchMore, hasMore, lastCursor, limit, loadingMore]);

  const refetchBuilds = useCallback(async () => {
    if (!buildSourceId) return [];
    setExhausted({ id: buildSourceId, done: false });
    const res = await refetch();
    return (res.data?.builds ?? []).flatMap((raw) => {
      const mapped = mapBuildSourceBuild(raw, buildSourceId);
      return mapped ? [mapped] : [];
    });
  }, [buildSourceId, refetch]);

  return {
    builds,
    loading: Boolean(enabled && buildSourceId) && loading && data === undefined,
    loadingMore,
    hasMore,
    error,
    loadMore,
    refetch: refetchBuilds,
  };
}

export interface UseBuildSourceBuildResult {
  build: BuildSourceBuildView | null;
  loading: boolean;
  error: Error | undefined;
  notFound: boolean;
  refetch: () => Promise<BuildSourceBuildView | null>;
}

/**
 * Reads a single build (`bld-…`) of a Build Source, polling while the build is
 * still in progress so status, timestamps, and fan-out deploys update live.
 */
export function useBuildSourceBuild(
  buildSourceId: string | null,
  buildId: string | null,
  options: { poll?: boolean } = {},
): UseBuildSourceBuildResult {
  const poll = options.poll ?? true;
  const [activePoll, setActivePoll] = useState(false);

  const { data, loading, error, refetch } = useQuery(BuildSourceBuildDocument, {
    variables: { buildSourceId: buildSourceId ?? "", id: buildId ?? "" },
    skip: !buildSourceId || !buildId,
    fetchPolicy: "cache-and-network",
    nextFetchPolicy: "cache-first",
    errorPolicy: "all",
    pollInterval: poll && activePoll ? ACTIVE_BUILD_POLL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });

  const build = useMemo(
    () => mapBuildSourceBuild(data?.build, buildSourceId ?? ""),
    [data, buildSourceId],
  );

  const isActive = build != null && isActiveBuildStatus(build.status);
  if (poll && activePoll !== isActive) {
    setActivePoll(isActive);
  }

  const refetchOne = useCallback(async () => {
    if (!buildSourceId || !buildId) return null;
    const res = await refetch();
    return mapBuildSourceBuild(res.data?.build, buildSourceId);
  }, [buildId, buildSourceId, refetch]);

  return {
    build,
    loading: Boolean(buildSourceId && buildId) && loading && data === undefined,
    error,
    notFound: isNotFoundError(error),
    refetch: refetchOne,
  };
}

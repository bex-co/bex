import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  BuildSourceBuildDocument,
  BuildSourceBuildsDocument,
} from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import {
  mapBuildSourceBuild,
  type BuildSourceBuildView,
} from "@/features/build-sources/types";

export interface UseBuildSourceBuildsResult {
  builds: BuildSourceBuildView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<BuildSourceBuildView[]>;
}

/** Lists builds produced by a Build Source (newest first). */
export function useBuildSourceBuilds(
  buildSourceId: string | null,
  enabled = true,
  limit = 50,
): UseBuildSourceBuildsResult {
  const { data, loading, error, refetch } = useQuery(
    BuildSourceBuildsDocument,
    {
      variables: { buildSourceId: buildSourceId ?? "", limit },
      skip: !enabled || !buildSourceId,
      fetchPolicy: "cache-and-network",
      nextFetchPolicy: "cache-first",
      errorPolicy: "all",
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

  const refetchBuilds = useCallback(async () => {
    if (!buildSourceId) return [];
    const res = await refetch();
    return (res.data?.builds ?? []).flatMap((raw) => {
      const mapped = mapBuildSourceBuild(raw, buildSourceId);
      return mapped ? [mapped] : [];
    });
  }, [buildSourceId, refetch]);

  return {
    builds,
    loading: Boolean(enabled && buildSourceId) && loading && data === undefined,
    error,
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

/** Reads a single build (`bld-…`) of a Build Source. */
export function useBuildSourceBuild(
  buildSourceId: string | null,
  buildId: string | null,
): UseBuildSourceBuildResult {
  const { data, loading, error, refetch } = useQuery(BuildSourceBuildDocument, {
    variables: { buildSourceId: buildSourceId ?? "", id: buildId ?? "" },
    skip: !buildSourceId || !buildId,
    fetchPolicy: "cache-and-network",
    nextFetchPolicy: "cache-first",
    errorPolicy: "all",
  });

  const build = useMemo(
    () => mapBuildSourceBuild(data?.build, buildSourceId ?? ""),
    [data, buildSourceId],
  );

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

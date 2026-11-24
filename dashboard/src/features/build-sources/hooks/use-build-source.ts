import { useCallback, useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { BuildSourceDocument } from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import {
  mapBuildSource,
  type BuildSourceView,
} from "@/features/build-sources/types";

export interface UseBuildSourceResult {
  buildSource: BuildSourceView | null;
  loading: boolean;
  error: Error | undefined;
  notFound: boolean;
  refetch: () => Promise<BuildSourceView | null>;
}

/** Reads a single Build Source by id (null/empty skips). */
export function useBuildSource(id: string | null): UseBuildSourceResult {
  const { data, loading, error, refetch } = useQuery(BuildSourceDocument, {
    variables: { id: id ?? "" },
    skip: !id,
    fetchPolicy: "cache-and-network",
    nextFetchPolicy: "cache-first",
    errorPolicy: "all",
  });

  const buildSource = useMemo(
    () => mapBuildSource(data?.buildSource),
    [data],
  );

  const refetchOne = useCallback(async () => {
    if (!id) return null;
    const res = await refetch();
    return mapBuildSource(res.data?.buildSource);
  }, [id, refetch]);

  return {
    buildSource,
    loading: Boolean(id) && loading && data === undefined,
    error,
    notFound: isNotFoundError(error),
    refetch: refetchOne,
  };
}

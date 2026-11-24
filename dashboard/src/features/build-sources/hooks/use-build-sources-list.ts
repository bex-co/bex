import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@apollo/client/react";
import { BuildSourcesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  mapBuildSource,
  type BuildSourceView,
} from "@/features/build-sources/types";

/** Render's Build Sources page cap. */
export const BUILD_SOURCES_PAGE_LIMIT = 100;

export interface UseBuildSourcesListOptions {
  /** Set false to defer fetching until a dialog or tab opens. Defaults to true. */
  enabled?: boolean;
  /** Optional project filter (applied client-side over the shared workspace cache). */
  projectId?: string | null;
}

export interface UseBuildSourcesListResult {
  buildSources: BuildSourceView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<BuildSourceView[]>;
}

/**
 * Lists every Build Source in the active workspace, paging 100 items at a time
 * (Render's page cap) so pickers, `/build-sources`, `⌘K`, and Project Overview
 * all share one Apollo cache entry keyed by `{ ownerId }`.
 */
export function useBuildSourcesList({
  enabled = true,
  projectId = null,
}: UseBuildSourcesListOptions = {}): UseBuildSourcesListResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, fetchMore, refetch } = useQuery(
    BuildSourcesDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      skip: !enabled || !resolved,
      fetchPolicy: "cache-and-network",
      nextFetchPolicy: "cache-first",
      errorPolicy: "all",
    },
  );

  // A full page means there may be more: fetch the next one and merge by id,
  // tracking each cursor once per workspace so a repeated render cannot loop.
  const pages = data?.buildSources ?? [];
  const lastCursor = pages.at(-1)?.cursor ?? null;
  const more =
    pages.length > 0 && pages.length % BUILD_SOURCES_PAGE_LIMIT === 0;
  const fetched = useRef<{ workspaceId: string | null; cursors: Set<string> }>({
    workspaceId: currentWorkspaceId,
    cursors: new Set<string>(),
  });
  const [settled, setSettled] = useState<string | null>(null);

  useEffect(() => {
    if (fetched.current.workspaceId !== currentWorkspaceId) {
      fetched.current = {
        workspaceId: currentWorkspaceId,
        cursors: new Set<string>(),
      };
    }
    if (
      !enabled ||
      !more ||
      !lastCursor ||
      loading ||
      fetched.current.cursors.has(lastCursor)
    ) {
      return;
    }
    fetched.current.cursors.add(lastCursor);
    fetchMore({
      variables: { cursor: lastCursor },
      updateQuery: (prev, { fetchMoreResult }) => {
        const seen = new Set((prev.buildSources ?? []).map((b) => b?.id));
        return {
          ...prev,
          buildSources: [
            ...(prev.buildSources ?? []),
            ...(fetchMoreResult.buildSources ?? []).filter(
              (b) => !seen.has(b?.id),
            ),
          ],
        };
      },
    })
      .catch(() => {
        // A failed page ends the walk; callers keep the rows already fetched.
      })
      .finally(() => setSettled(lastCursor));
  }, [currentWorkspaceId, enabled, more, lastCursor, loading, fetchMore]);

  const allSources = useMemo(
    () =>
      (data?.buildSources ?? []).flatMap((raw) => {
        const mapped = mapBuildSource(raw);
        return mapped ? [mapped] : [];
      }),
    [data],
  );

  const buildSources = useMemo(
    () =>
      projectId
        ? allSources.filter((b) => b.projectId === projectId)
        : allSources,
    [allSources, projectId],
  );

  const refetchList = useCallback(async () => {
    fetched.current.cursors.clear();
    const res = await refetch();
    const mapped = (res.data?.buildSources ?? []).flatMap((raw) => {
      const item = mapBuildSource(raw);
      return item ? [item] : [];
    });
    return projectId ? mapped.filter((b) => b.projectId === projectId) : mapped;
  }, [refetch, projectId]);

  const pendingMore = more && lastCursor !== null && settled !== lastCursor;
  // Gate initial loading on having no data yet so background refetches never
  // unmount open dialogs or lose drafts (`dashboard/AGENTS.md`).
  const initialLoading =
    enabled && (!resolved || (loading && data === undefined) || pendingMore);

  return {
    buildSources,
    loading: initialLoading,
    error,
    refetch: refetchList,
  };
}

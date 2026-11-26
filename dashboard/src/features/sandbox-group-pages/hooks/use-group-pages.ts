import { useCallback, useEffect, useMemo, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import {
  DeleteGroupSandboxSnapshotDocument,
  SandboxBuildSourcesDocument,
  SandboxDetailsDocument,
  SandboxEventsDocument,
  SandboxGroupSandboxesPageDocument,
  SandboxImageVersionsDocument,
  SandboxSnapshotDetailsDocument,
  SandboxSnapshotsForGroupDocument,
  SandboxesForGroupDocument,
  TerminateGroupSandboxDocument,
  type SandboxBuildSourcesQuery,
  type SandboxEventsQuery,
  type SandboxListFieldsFragment,
  type SandboxSnapshotRowFieldsFragment,
} from "@/graphql/definitions";
import { isNotFoundError } from "@/common/lib/graphql-error";
import {
  appendPage,
  joinAppendedPages,
  NO_APPENDED,
  type AppendedPages,
} from "@/common/lib/appended-pages";
import { skipPollWhenHidden } from "@/common/lib/polling";
import {
  rangeStart,
  isEndedSandbox,
  type SandboxStatus,
  type TimeRange,
} from "@/features/sandbox-group-pages/lib/sandbox-view";

/**
 * The reads and writes of a sandbox group's pages (w1/m658). Render keeps
 * them live over subscriptions; bex polls instead: the Sandboxes list, its
 * summary and the running count every 5 s while live updates are on, an open
 * drawer every 5 s while its sandbox runs, and the Snapshots and Custom images
 * pages every 10 s while something on them is still settling.
 */

/** How often a live list and an open drawer re-read. */
export const LIVE_POLL_MS = 5_000;
/** How often the Snapshots and Custom images pages re-read while one settles. */
export const SETTLING_POLL_MS = 10_000;
/** Rows per page, as Render's list pages. */
export const PAGE_SIZE = 25;

export type SandboxRow = SandboxListFieldsFragment;
export type SnapshotRow = SandboxSnapshotRowFieldsFragment;
export type SandboxEventRow =
  SandboxEventsQuery["sandboxEvents"]["events"][number];
export type ImageSourceRow =
  SandboxBuildSourcesQuery["sandboxBuildSources"]["sources"][number];

/**
 * The start of the selected range, its "now" re-anchored every minute while
 * live, so a poll's question changes at most once a minute.
 */
function useRangeStart(range: TimeRange, live: boolean): string | undefined {
  const [anchor, setAnchor] = useState(() => Date.now());
  useEffect(() => {
    if (!live || range === "all") return;
    const id = setInterval(() => setAnchor(Date.now()), 60_000);
    return () => clearInterval(id);
  }, [live, range]);
  return rangeStart(range, anchor);
}

export interface GroupSandboxesQuery {
  groupId: string;
  ownerId: string;
  search: string;
  status: SandboxStatus | null;
  range: TimeRange;
  cursor: string | undefined;
  live: boolean;
}

/**
 * One page of the group's sandboxes, its run summary and running count: one
 * document, polled every 5 s while live. It is not cached (`no-cache`): its
 * range start moves every minute, and each start would keep an entry; the
 * last answer stays on screen while the next loads.
 */
export function useGroupSandboxes(q: GroupSandboxesQuery) {
  const createdAfter = useRangeStart(q.range, q.live);
  const query = useQuery(SandboxGroupSandboxesPageDocument, {
    variables: {
      id: q.groupId,
      ownerId: q.ownerId,
      endedAfter: createdAfter,
      input: {
        sandboxGroupId: q.groupId,
        ownerId: q.ownerId,
        statuses: q.status ? [q.status] : undefined,
        search: q.status ? undefined : q.search.trim() || undefined,
        createdAfter,
        cursor: q.cursor,
        limit: PAGE_SIZE,
      },
    },
    fetchPolicy: "no-cache",
    errorPolicy: "all",
    pollInterval: q.live ? LIVE_POLL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });
  const data = query.data ?? query.previousData;
  const page = data?.sandboxesForGroup;
  return {
    sandboxes: page?.sandboxes ?? [],
    cursor: page?.cursor ?? null,
    hasMore: page?.hasMore ?? false,
    // Loading only until the first answer: a poll never unmounts the rows.
    loading: query.loading && data === undefined,
    error: data === undefined ? query.error : undefined,
    runs: data?.sandboxGroupRuns ?? null,
    running: data?.sandboxGroup?.runningSandboxCount ?? null,
    refetch: query.refetch,
  };
}

/** Terminates sandboxes, each on its own; resolves the ids that failed. */
export function useTerminateSandboxes(ownerId: string) {
  const [mutate] = useMutation(TerminateGroupSandboxDocument);
  const [pending, setPending] = useState(false);
  const terminate = useCallback(
    async (ids: string[]): Promise<string[]> => {
      setPending(true);
      try {
        const results = await Promise.allSettled(
          ids.map((id) => mutate({ variables: { id, ownerId } })),
        );
        return ids.filter(
          (_, i) =>
            results[i].status === "rejected" ||
            (results[i] as PromiseFulfilledResult<{ error?: unknown }>).value
              ?.error,
        );
      } finally {
        setPending(false);
      }
    },
    [mutate, ownerId],
  );
  return { terminate, pending };
}

/**
 * An open sandbox drawer: the sandbox, polled until it ends (a suspended one
 * can still resume).
 */
export function useSandboxDetails(id: string, ownerId: string) {
  const query = useQuery(SandboxDetailsDocument, {
    variables: { id, ownerId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: LIVE_POLL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const sandbox = query.data?.sandbox ?? null;
  const ended = sandbox !== null && isEndedSandbox(sandbox.status);
  const { stopPolling } = query;
  useEffect(() => {
    if (ended) stopPolling();
  }, [ended, stopPolling]);
  return {
    sandbox,
    loading: query.loading && query.data === undefined,
    notFound:
      isNotFoundError(query.error) || (query.data !== undefined && !sandbox),
    error:
      query.data === undefined && !isNotFoundError(query.error)
        ? query.error
        : undefined,
    refetch: query.refetch,
  };
}

const EVENT_PAGE = 50;

/**
 * A sandbox's events, newest first: the newest page polled while the
 * sandbox runs, and earlier pages on demand, joined as other Load more lists
 * join theirs (common/lib/appended-pages.ts).
 */
export function useSandboxEvents(
  sandboxId: string,
  ownerId: string,
  live: boolean,
) {
  const client = useApolloClient();
  const head = useQuery(SandboxEventsDocument, {
    variables: { sandboxId, ownerId, limit: EVENT_PAGE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: live ? LIVE_POLL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });
  const [appended, setAppended] =
    useState<AppendedPages<SandboxEventRow>>(NO_APPENDED);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [earlierError, setEarlierError] = useState(false);
  const joined = useMemo(
    () =>
      joinAppendedPages(
        head.data?.sandboxEvents.events ?? [],
        appended,
        sandboxId,
        EVENT_PAGE,
      ),
    [appended, head.data, sandboxId],
  );

  const loadEarlier = useCallback(async () => {
    const next = joined.next;
    const last = joined.rows.find((row) => row.id === next?.cursor);
    if (!next || !last) return;
    setLoadingEarlier(true);
    setEarlierError(false);
    try {
      const { data, error } = await client.query({
        query: SandboxEventsDocument,
        variables: {
          sandboxId,
          ownerId,
          before: last.timestamp,
          beforeId: last.id,
          limit: EVENT_PAGE,
        },
        fetchPolicy: "network-only",
      });
      if (error || !data) throw error ?? new Error("no events");
      setAppended((prev) => appendPage(prev, next, data.sandboxEvents.events));
    } catch {
      setEarlierError(true);
    } finally {
      setLoadingEarlier(false);
    }
  }, [client, joined, ownerId, sandboxId]);

  return {
    events: joined.rows,
    hasMore: joined.hasMore,
    loading: head.loading && head.data === undefined,
    error: head.data === undefined ? head.error : undefined,
    loadEarlier,
    loadingEarlier,
    earlierError,
  };
}

export interface GroupSnapshotsQuery {
  groupId: string;
  ownerId: string;
  search?: string;
  range?: TimeRange;
  sourceSandboxId?: string;
  cursor?: string;
}

/** One page of the group's snapshots, polled while one is capturing. */
export function useGroupSnapshots(q: GroupSnapshotsQuery) {
  const createdAfter = useRangeStart(q.range ?? "all", true);
  const query = useQuery(SandboxSnapshotsForGroupDocument, {
    variables: {
      input: {
        sandboxGroupId: q.groupId,
        ownerId: q.ownerId,
        search: q.search?.trim() || undefined,
        createdAfter,
        sourceSandboxId: q.sourceSandboxId,
        cursor: q.cursor,
        limit: PAGE_SIZE,
      },
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const page = query.data?.sandboxSnapshotsForGroup;
  const snapshots = useMemo(() => page?.snapshots ?? [], [page]);
  useSettlingPoll(
    snapshots.some((s) => s.status === "creating"),
    query.startPolling,
    query.stopPolling,
  );
  return {
    snapshots,
    cursor: page?.cursor ?? null,
    hasMore: page?.hasMore ?? false,
    loading: query.loading && query.data === undefined,
    error: query.data === undefined ? query.error : undefined,
    refetch: query.refetch,
  };
}

/** One snapshot of the group by id, for a drawer whose page does not list it. */
export function useSnapshotDetails(
  groupId: string,
  ownerId: string,
  id: string,
  skip: boolean,
) {
  const query = useQuery(SandboxSnapshotDetailsDocument, {
    variables: { sandboxGroupId: groupId, id, ownerId },
    skip,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  return {
    snapshot: query.data?.sandboxSnapshot ?? null,
    loading: !skip && query.loading && query.data === undefined,
    error:
      query.data === undefined && !isNotFoundError(query.error)
        ? query.error
        : undefined,
    refetch: query.refetch,
  };
}

/** Deletes one snapshot; rejects with bex-api's refusal. */
export function useDeleteSnapshot(groupId: string, ownerId: string) {
  const [mutate, { loading }] = useMutation(DeleteGroupSandboxSnapshotDocument);
  const remove = useCallback(
    async (id: string) => {
      const { error } = await mutate({
        variables: { sandboxGroupId: groupId, id, ownerId },
      });
      if (error) throw error;
    },
    [groupId, mutate, ownerId],
  );
  return { remove, pending: loading };
}

/** One page of the group's images by name, polled while one imports. */
export function useGroupImageSources(
  groupId: string,
  ownerId: string,
  search: string,
  cursor: string | undefined,
) {
  const query = useQuery(SandboxBuildSourcesDocument, {
    variables: {
      input: {
        sandboxGroupId: groupId,
        ownerId,
        search: search.trim() || undefined,
        cursor,
        limit: PAGE_SIZE,
      },
    },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const page = query.data?.sandboxBuildSources;
  const sources = useMemo(() => page?.sources ?? [], [page]);
  useSettlingPoll(
    sources.some(
      (s) =>
        s.latestBuild.status === "pending" ||
        s.latestBuild.status === "running",
    ),
    query.startPolling,
    query.stopPolling,
  );
  return {
    sources,
    cursor: page?.cursor ?? null,
    hasMore: page?.hasMore ?? false,
    loading: query.loading && query.data === undefined,
    error: query.data === undefined ? query.error : undefined,
    refetch: query.refetch,
  };
}

/** Every version of one image name, newest first. */
export function useImageVersions(
  groupId: string,
  ownerId: string,
  name: string | null,
) {
  const query = useQuery(SandboxImageVersionsDocument, {
    variables: { sandboxGroupId: groupId, name: name ?? "", ownerId },
    skip: !name,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const versions = useMemo(
    () =>
      [...(query.data?.sandboxImages ?? [])]
        .filter((v) => v != null)
        .sort((a, b) => (b.createdAt ?? "").localeCompare(a.createdAt ?? "")),
    [query.data],
  );
  return {
    versions,
    loading: query.loading && query.data === undefined,
    error: query.data === undefined ? query.error : undefined,
    refetch: query.refetch,
  };
}

/** The sandboxes that started from a snapshot or an image's versions. */
export function useSandboxesStartedFrom(
  groupId: string,
  ownerId: string,
  source: { snapshotId?: string; buildIds?: string[] } | null,
) {
  const buildIds = source?.buildIds ?? [];
  const query = useQuery(SandboxesForGroupDocument, {
    variables: {
      input: {
        sandboxGroupId: groupId,
        ownerId,
        sourceSnapshotId: source?.snapshotId,
        sourceBuildIds: buildIds.length > 0 ? buildIds : undefined,
        limit: PAGE_SIZE,
      },
    },
    skip: !source || (!source.snapshotId && buildIds.length === 0),
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  return {
    sandboxes: query.data?.sandboxesForGroup.sandboxes ?? [],
    loading: query.loading && query.data === undefined,
    error: query.data === undefined ? query.error : undefined,
  };
}

/**
 * Cursor pages of a list: Previous and Next over the cursors read so far.
 * A new question resets to the first page.
 */
export function useCursorPages() {
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  return {
    cursor: cursors.at(-1),
    page: cursors.length - 1,
    next: (cursor: string | null) =>
      cursor && setCursors((prev) => [...prev, cursor]),
    previous: () =>
      setCursors((prev) => (prev.length > 1 ? prev.slice(0, -1) : prev)),
    reset: () => setCursors([undefined]),
  };
}

/** Re-read a page while something on it is still settling, and stop after. */
function useSettlingPoll(
  settling: boolean,
  startPolling: (ms: number) => void,
  stopPolling: () => void,
) {
  useEffect(() => {
    if (settling) startPolling(SETTLING_POLL_MS);
    else stopPolling();
  }, [settling, startPolling, stopPolling]);
}

/**
 * Selected rows of a list, read through the rows still on the page so a
 * refresh never acts on a sandbox the user can no longer see.
 */
export function useRowSelection(rowIds: string[]) {
  const [chosen, setChosen] = useState<Set<string>>(new Set());
  const selected = useMemo(
    () => new Set([...chosen].filter((id) => rowIds.includes(id))),
    [chosen, rowIds],
  );
  const toggle = useCallback((id: string, on: boolean) => {
    setChosen((prev) => {
      const next = new Set(prev);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);
  const toggleAll = useCallback((ids: string[]) => setChosen(new Set(ids)), []);
  const clear = useCallback(() => setChosen(new Set()), []);
  return { selected, toggle, toggleAll, clear };
}

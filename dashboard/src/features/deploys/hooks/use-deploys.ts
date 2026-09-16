import { useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { DeploysDocument, type DeploysQuery } from "@/graphql/definitions";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import { isTerminalDeployStatus } from "@/features/deploys/lib/deploy-status";

const PAGE_SIZE = 20;

/** Unit separator — keeps filterKey unambiguous without a null byte. */
const FILTER_SEP = "\u001f";

export interface DeployRow {
  id: string;
  status: string;
  trigger: string;
  image: string;
  rollbackOf: string;
  commitId: string;
  commitMessage: string;
  commitCreatedAt: string | null;
  createdAt: string | null;
  updatedAt: string | null;
  startedAt: string | null;
  finishedAt: string | null;
  preDeployStatus: string;
  /** Actionable cause of a failed deploy (w1/m138); "" unless it failed. */
  failureReason: string;
  /** Neutral cause of a non-user cancel (w4/089); "" unless superseded. */
  cancelReason: string;
}

type RawDeploy = NonNullable<DeploysQuery["deploys"]>[number];

function toRows(raw: DeploysQuery["deploys"] | undefined): DeployRow[] {
  return (raw ?? [])
    .filter((d): d is NonNullable<RawDeploy> & { id: string } => !!d?.id)
    .map((d) => ({
      id: d.id,
      status: d.status ?? "",
      trigger: d.trigger ?? "",
      image: d.image ?? "",
      rollbackOf: d.rollbackOf ?? "",
      commitId: d.commitId ?? "",
      commitMessage: d.commitMessage ?? "",
      commitCreatedAt: d.commitCreatedAt ?? null,
      createdAt: d.createdAt ?? null,
      updatedAt: d.updatedAt ?? null,
      startedAt: d.startedAt ?? null,
      finishedAt: d.finishedAt ?? null,
      preDeployStatus: d.preDeployStatus ?? "",
      failureReason: d.failureReason ?? "",
      cancelReason: d.cancelReason ?? "",
    }));
}

export interface UseDeploysResult {
  deploys: DeployRow[];
  loading: boolean;
  loadingMore: boolean;
  error: Error | undefined;
  hasMore: boolean;
  loadMore: () => void;
}

/** The `loadMore`-appended pages, tagged with the filter they were read for. */
interface AppendedPages {
  filterKey: string;
  rows: DeployRow[];
  /** Size of the most recently appended page (null ⇒ none appended yet). */
  lastPageSize: number | null;
  error: Error | undefined;
}

const NO_APPENDED: AppendedPages = {
  filterKey: "",
  rows: [],
  lastPageSize: null,
  error: undefined,
};

/**
 * Merge page-1 with appended history, dropping duplicates by id. A poll that
 * prepends a new deploy shifts the keyset so page 1 overlaps an appended page —
 * in that case drop the stale appends the way a filter change already does
 * (w4/073).
 */
export function mergeDeployPages(
  firstPage: DeployRow[],
  appended: DeployRow[],
): { rows: DeployRow[]; dropAppended: boolean } {
  const firstIds = new Set(firstPage.map((d) => d.id));
  if (appended.some((d) => firstIds.has(d.id))) {
    return { rows: firstPage, dropAppended: true };
  }
  const seen = new Set(firstIds);
  const rows = [...firstPage];
  for (const d of appended) {
    if (seen.has(d.id)) continue;
    seen.add(d.id);
    rows.push(d);
  }
  return { rows, dropAppended: false };
}

/**
 * Reads a service's deploy history (`deploys(serviceId, status[], cursor,
 * limit)`) newest-first for the dedicated Deploys tab (w9/002). `deploys` is
 * a bare list with no Apollo pagination field policy — the use-audit-log
 * precedent — so `loadMore` re-queries imperatively past the last-loaded
 * deploy's id (the surface's keyset cursor, internal/store's pageNewestFirst)
 * and appends the pages itself. Appended pages carry the status filter they
 * were read for, so changing the filter drops them (and any in-flight page
 * that lands late) instead of stacking one filter's tail under another's
 * first page.
 *
 * Polls on the same converging cadence as `useLatestDeploy` (w4/073 / the
 * mirror of w6/m46 t005): arrivals while watching the tab must appear without
 * a reload. Status updates on rows already in the list can ride the header's
 * limit:1 entity rewrite; new ids cannot — only this poll refreshes the
 * limit:20 array.
 */
export function useDeploys(
  serviceId: string,
  statuses: string[],
): UseDeploysResult {
  const client = useApolloClient();
  const status = statuses.length > 0 ? statuses : undefined;
  const filterKey = `${serviceId}${FILTER_SEP}${statuses.join(",")}`;

  const { data, loading, error, startPolling, stopPolling } = useQuery(
    DeploysDocument,
    {
      variables: { serviceId, status, limit: PAGE_SIZE },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const firstPage = useMemo(() => toRows(data?.deploys), [data]);

  const [appended, setAppended] = useState<AppendedPages>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);

  const scoped = useMemo(() => {
    if (appended.filterKey !== filterKey) return NO_APPENDED;
    const firstIds = new Set(firstPage.map((d) => d.id));
    if (appended.rows.some((d) => firstIds.has(d.id))) return NO_APPENDED;
    return appended;
  }, [appended, filterKey, firstPage]);

  const deploys = useMemo(
    () => mergeDeployPages(firstPage, scoped.rows).rows,
    [firstPage, scoped.rows],
  );

  // Not yet loaded counts as converging, matching useLatestDeploy.
  useConvergingPoll(
    startPolling,
    stopPolling,
    deploys.length === 0 ||
      deploys.some((d) => !isTerminalDeployStatus(d.status)),
  );

  const hasMore =
    scoped.lastPageSize === null
      ? firstPage.length === PAGE_SIZE
      : scoped.lastPageSize === PAGE_SIZE;

  async function loadMore() {
    if (loadingMore || deploys.length === 0) return;
    const key = filterKey;
    setLoadingMore(true);
    try {
      const cursor = deploys[deploys.length - 1].id;
      const result = await client.query({
        query: DeploysDocument,
        variables: { serviceId, status, cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      const page = toRows(result.data?.deploys);
      // Defensive only — the exclusive keyset cursor shouldn't hand back an
      // id already loaded. Bounded to the current tail, not the full history.
      const seenTail = new Set(deploys.slice(-PAGE_SIZE).map((d) => d.id));
      setAppended((prev) => {
        const base = prev.filterKey === key ? prev.rows : [];
        return {
          filterKey: key,
          rows: [...base, ...page.filter((d) => !seenTail.has(d.id))],
          lastPageSize: page.length,
          error: result.error,
        };
      });
    } catch (err) {
      const failure = err instanceof Error ? err : new Error(String(err));
      setAppended((prev) => ({
        ...(prev.filterKey === key ? prev : NO_APPENDED),
        filterKey: key,
        error: failure,
      }));
    } finally {
      setLoadingMore(false);
    }
  }

  return {
    deploys,
    loading: loading && deploys.length === 0,
    loadingMore,
    error: error ?? scoped.error,
    hasMore,
    loadMore: () => void loadMore(),
  };
}

import { useMemo, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import { DeploysDocument, type DeploysQuery } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import { isTerminalDeployStatus } from "@/features/deploys/lib/deploy-status";
import {
  NO_APPENDED,
  appendPage,
  joinAppendedPages,
  type AppendRequest,
  type AppendedPages,
} from "@/common/lib/appended-pages";

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
  /**
   * Why an OPEN deploy is not progressing (w4/m112) — the operator's live
   * diagnosis of the current revision's pods, e.g. a failing health check
   * naming its path. "" while a rollout is healthy, and cleared by the server
   * the moment the deploy goes terminal. An observation, not a verdict.
   */
  stallReason: string;
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
      stallReason: d.stallReason ?? "",
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

/**
 * Reads a service's deploy history (`deploys(serviceId, status[], cursor,
 * limit)`) newest-first for the dedicated Deploys tab (w9/002). `deploys` is
 * a bare list with no Apollo pagination field policy — the use-audit-log
 * precedent — so `loadMore` re-queries imperatively past the last-loaded
 * deploy's id (the surface's keyset cursor, internal/store's pageNewestFirst)
 * and appends the pages itself. Appended pages carry the status filter they
 * were read for and the first-page boundary they hang off
 * (common/lib/appended-pages), so a filter change or a new deploy shifting
 * page 1 drops them — and the next Load more reads from the current head —
 * instead of stacking a stale tail under it with a row missing between
 * (w4/232).
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
  const { t } = useTranslations();
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

  const [appended, setAppended] =
    useState<AppendedPages<DeployRow>>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);

  const {
    rows: deploys,
    hasMore,
    next,
  } = useMemo(
    () => joinAppendedPages(firstPage, appended, filterKey, PAGE_SIZE),
    [appended, filterKey, firstPage],
  );
  // Not yet loaded counts as converging, matching useLatestDeploy.
  useConvergingPoll(
    startPolling,
    stopPolling,
    deploys.length === 0 ||
      deploys.some((d) => !isTerminalDeployStatus(d.status)),
  );

  async function loadMore() {
    if (loadingMore || !next) return;
    const request: AppendRequest = next;
    setLoadingMore(true);
    try {
      const result = await client.query({
        query: DeploysDocument,
        variables: {
          serviceId,
          status,
          cursor: request.cursor,
          limit: PAGE_SIZE,
        },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      // A partial page is still kept; an unreadable one is a failure, never a
      // successful empty page that would claim the history is exhausted.
      if (result.data?.deploys) {
        const page = toRows(result.data.deploys);
        setAppended((prev) => appendPage(prev, request, page));
      }
      if (result.error || !result.data?.deploys) throw result.error;
    } catch {
      // The page's own error state covers only an empty list, so an older-page
      // failure is announced here.
      toast.error(t("deploys.listLoadMoreError"));
    } finally {
      setLoadingMore(false);
    }
  }

  return {
    deploys,
    loading: loading && deploys.length === 0,
    loadingMore,
    error,
    hasMore,
    loadMore: () => void loadMore(),
  };
}

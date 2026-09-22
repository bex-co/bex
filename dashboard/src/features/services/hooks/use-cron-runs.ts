import { useCallback, useEffect, useMemo, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelCronJobRunDocument,
  CronJobRunsDocument,
  RunCronJobDocument,
  type CronJobRunsQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import type { CronRunView } from "@/features/services/types";

const PAGE_SIZE = 5;

// A run that hasn't reached a terminal status yet — used to gate Trigger Run
// (the operator forbids concurrent runs, so the button is disabled while one of
// these exists) and mirrored by the backend's own ForbidConcurrent rejection.
const ACTIVE_STATUSES = new Set(["pending", "running"]);

export interface UseCronRunsResult {
  runs: CronRunView[];
  loading: boolean;
  error: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  cancelingId: string | null;
  loadMore: () => Promise<void>;
  cancel: (runId: string) => Promise<boolean>;
  /** True while a run is pending/running — Trigger Run is disabled then. */
  hasActiveRun: boolean;
  triggering: boolean;
  /** The backend's rejection message from the last failed trigger, shown inline. */
  triggerError: string | null;
  clearTriggerError: () => void;
  trigger: () => Promise<boolean>;
}

/** The `loadMore`-appended pages, tagged with the service they were read for. */
interface AppendedPages {
  serviceId: string;
  rows: CronRunView[];
  /**
   * The id the FIRST appended page was read from — the row the tail hangs off.
   * While it is still on page 1, head and tail are contiguous; once it is not,
   * the refreshed head has moved past it and the two no longer join up.
   */
  anchorId: string;
  /** Size of the most recently appended page (null ⇒ none appended yet). */
  lastPageSize: number | null;
}

const NO_APPENDED: AppendedPages = {
  serviceId: "",
  rows: [],
  anchorId: "",
  lastPageSize: null,
};

/**
 * Merge page 1 with appended history, dropping duplicates by id.
 *
 * The appended tail is dropped in two cases, both of which mean it no longer
 * joins the refreshed head:
 *
 *   - **Overlap** — page 1 already contains an appended row, so keeping both
 *     would list it twice (the reconciliation `mergeDeployPages` performs for
 *     Deploys, w4/073).
 *   - **Gap** — the anchor the tail was read from has fallen off page 1, which
 *     is what a new arrival does: head [9…5] + tail [4…1] becomes head [10…6],
 *     and crr-5 belongs to neither. Concatenating anyway would silently hide a
 *     run in the middle of the history, with `hasMore` already false so Load
 *     more could not bring it back.
 *
 * Dropping restores the plain first page and, with it, a working Load more —
 * the tail is re-fetchable, which a hole in the middle is not. It also bounds
 * what is held: rows are never accumulated beyond the authoritative history.
 */
export function mergeCronRunPages(
  firstPage: CronRunView[],
  appended: CronRunView[],
  anchorId = "",
): { rows: CronRunView[]; dropAppended: boolean } {
  const firstIds = new Set(firstPage.map((run) => run.id));
  const overlaps = appended.some((run) => firstIds.has(run.id));
  const gapped =
    appended.length > 0 && anchorId !== "" && !firstIds.has(anchorId);
  if (overlaps || gapped) {
    return { rows: firstPage, dropAppended: true };
  }
  const seen = new Set(firstIds);
  const rows = [...firstPage];
  for (const run of appended) {
    if (seen.has(run.id)) continue;
    seen.add(run.id);
    rows.push(run);
  }
  return { rows, dropAppended: false };
}

type RawCronRun = NonNullable<CronJobRunsQuery["cronJobRuns"]>[number];

function toRuns(
  raw: CronJobRunsQuery["cronJobRuns"] | undefined,
): CronRunView[] {
  return (raw ?? [])
    .filter((run): run is NonNullable<RawCronRun> => run != null && !!run.id)
    .map((run) => ({
      id: run.id ?? "",
      startedAt: run.startedAt ?? null,
      finishedAt: run.finishedAt ?? null,
      status: run.status ?? "pending",
    }));
}

/**
 * Cursor-paged cron history + cancellation over the dedicated run-object API.
 *
 * Polls page 1 on the shared visible-tab cadence, because the operator creates
 * scheduled runs with no dashboard mutation to hang a refetch on. Without this,
 * a mounted Recent Runs table showed its original rows indefinitely while the
 * API already reported newer successful executions — only a reload added them
 * (w4/138). Apollo's Server poll cannot cover it: it rewrites CronRun ENTITIES
 * already referenced by this list, but nothing inserts a new reference into the
 * root `cronJobRuns` array.
 *
 * The baseline tick runs even with no active run, which is the case that
 * matters — a settled history is exactly the state a new scheduled run arrives
 * into.
 *
 * Appended pages live in React state rather than being concatenated into the
 * cache entry with `fetchMore`. That is load-bearing, not a style choice: a
 * poll writes a fresh five-row first page under the original variables, so a
 * merged ten-row entry would be truncated back to five on the next tick,
 * silently dropping loaded older rows and their Load more affordance.
 */
export function useCronRuns(serviceId: string): UseCronRunsResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [appendedState, setAppendedState] =
    useState<AppendedPages>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);
  const [cancelingId, setCancelingId] = useState<string | null>(null);
  const { data, loading, error, refetch, startPolling, stopPolling } = useQuery(
    CronJobRunsDocument,
    {
      variables: { serviceId, limit: PAGE_SIZE },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const [cancelRun] = useMutation(CancelCronJobRunDocument);
  const [runCronJob] = useMutation(RunCronJobDocument);
  const [triggering, setTriggering] = useState(false);
  const [triggerError, setTriggerError] = useState<string | null>(null);

  // Last good first page, kept so a transient poll failure leaves the history
  // standing instead of blanking it. With errorPolicy "all" a failed refresh
  // delivers no data at all, and the live report included exactly such a blip
  // (one ERR_CONNECTION_CLOSED between two successful reads) — a refresh
  // mechanism that empties the table on those is worse than no refresh.
  // Tagged with the service so another cron never shows this one's rows.
  const [retained, setRetained] = useState<{
    serviceId: string;
    rows: CronRunView[];
  }>({ serviceId: "", rows: [] });

  const fetched = useMemo(() => toRuns(data?.cronJobRuns), [data]);
  const hasFetched = !!data?.cronJobRuns;

  useEffect(() => {
    if (hasFetched) setRetained({ serviceId, rows: fetched });
  }, [hasFetched, fetched, serviceId]);

  const firstPage = useMemo(
    () =>
      hasFetched
        ? fetched
        : retained.serviceId === serviceId
          ? retained.rows
          : [],
    [hasFetched, fetched, retained, serviceId],
  );

  const { runs, scoped } = useMemo(() => {
    // Appends are tagged with the service they were read for, so navigating to
    // another cron drops them instead of stacking one job's tail under
    // another's first page.
    const pages =
      appendedState.serviceId === serviceId ? appendedState : NO_APPENDED;
    const { rows, dropAppended } = mergeCronRunPages(
      firstPage,
      pages.rows,
      pages.anchorId,
    );
    return { runs: rows, scoped: dropAppended ? NO_APPENDED : pages };
  }, [appendedState, serviceId, firstPage]);

  const hasActiveRun = runs.some((run) =>
    ACTIVE_STATUSES.has(run.status.toLowerCase()),
  );

  // Fast while a run is in flight, baseline otherwise — the baseline tick is
  // what discovers a scheduled run in a settled history.
  useConvergingPoll(startPolling, stopPolling, hasActiveRun);

  const hasMore =
    scoped.lastPageSize === null
      ? firstPage.length === PAGE_SIZE
      : scoped.lastPageSize === PAGE_SIZE;

  const loadMore = useCallback(async () => {
    if (loadingMore || runs.length === 0) return;
    const key = serviceId;
    const cursor = runs[runs.length - 1]?.id;
    if (!cursor) return;
    setLoadingMore(true);
    try {
      const result = await client.query({
        query: CronJobRunsDocument,
        variables: { serviceId, cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      const page = toRuns(result.data?.cronJobRuns);
      // Defensive only — the exclusive keyset cursor shouldn't hand back an id
      // already loaded. Bounded to the current tail, not the full history.
      const seenTail = new Set(runs.slice(-PAGE_SIZE).map((run) => run.id));
      setAppendedState((prev) => {
        const carried = prev.serviceId === key ? prev : NO_APPENDED;
        return {
          serviceId: key,
          rows: [
            ...carried.rows,
            ...page.filter((run) => !seenTail.has(run.id)),
          ],
          // The anchor is the FIRST page's cursor and never moves as more
          // pages are appended: it marks where the tail joins the head.
          anchorId: carried.anchorId || cursor,
          lastPageSize: page.length,
        };
      });
    } catch {
      toast.error(t("services.cronRunsLoadError"));
    } finally {
      setLoadingMore(false);
    }
  }, [client, loadingMore, runs, serviceId, t]);

  const cancel = useCallback(
    async (runId: string) => {
      setCancelingId(runId);
      try {
        await cancelRun({ variables: { serviceId, runId } });
        toast.success(t("services.cronRunCancelSuccess"));
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("services.cronRunCancelError")),
        );
        return false;
      } finally {
        setCancelingId(null);
      }
    },
    [cancelRun, serviceId, t],
  );

  const trigger = useCallback(async () => {
    setTriggering(true);
    setTriggerError(null);
    try {
      await runCronJob({ variables: { id: serviceId } });
      // A fresh first-page read surfaces the new pending run at the top.
      await refetch();
      toast.success(t("services.cronTriggerSuccess"));
      return true;
    } catch (e) {
      // Surface the backend's rejection (e.g. an already-active run under
      // ForbidConcurrent) inline rather than swallowing it in a toast.
      setTriggerError(
        e instanceof Error && e.message
          ? e.message
          : t("services.cronTriggerError"),
      );
      return false;
    } finally {
      setTriggering(false);
    }
  }, [runCronJob, refetch, serviceId, t]);

  return {
    runs,
    // Don't flash a skeleton over rows we already have while a background
    // refresh is in flight (cache-and-network reports loading on every tick).
    loading: loading && runs.length === 0,
    error: !!error,
    loadingMore,
    hasMore,
    cancelingId,
    loadMore,
    cancel,
    hasActiveRun,
    triggering,
    triggerError,
    clearTriggerError: () => setTriggerError(null),
    trigger,
  };
}

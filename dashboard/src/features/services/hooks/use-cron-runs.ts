import { useCallback, useMemo, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelCronJobRunDocument,
  CronJobRunsDocument,
  type CronJobRunsQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import {
  NO_APPENDED,
  appendPage,
  joinAppendedPages,
  type AppendedPages,
} from "@/common/lib/appended-pages";
import type { CronRunView } from "@/features/services/types";
import { CRON_RUNS_PAGE_SIZE as PAGE_SIZE } from "@/features/services/lib/cron-run-list";

// A run that hasn't reached a terminal status yet: the history polls fast
// while one is listed.
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
}

type RawCronRun = NonNullable<CronJobRunsQuery["cronJobRuns"]>[number];

function toRuns(
  raw: CronJobRunsQuery["cronJobRuns"] | undefined,
): CronRunView[] {
  return (raw ?? [])
    .filter((run): run is NonNullable<RawCronRun> => run != null && !!run.id)
    .map((run) => ({
      id: run.id ?? "",
      // `||`, not `??`: an older server answered "" for a run that had not
      // started, which must read as absent (w4/m300).
      startedAt: run.startedAt || null,
      finishedAt: run.finishedAt || null,
      status: run.status ?? "pending",
      triggeredBy: run.triggeredBy || null,
      canceledBy: run.canceledBy || null,
    }));
}

/**
 * Cursor-paged cron history + cancellation over the dedicated run-object API.
 * A page is Render's 30 runs; Load more reads 30 older ones (w3/m556).
 *
 * Polls page 1 on the shared visible-tab cadence, because the operator creates
 * scheduled runs with no dashboard mutation to hang a refetch on. Without this,
 * a mounted runs table showed its original rows indefinitely while the
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
 * poll writes a fresh first page under the original variables, so a
 * merged two-page entry would be truncated back to one page on the next tick,
 * silently dropping loaded older rows and their Load more affordance.
 */
export function useCronRuns(serviceId: string): UseCronRunsResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [appendedState, setAppendedState] =
    useState<AppendedPages<CronRunView>>(NO_APPENDED);
  const [loadingMore, setLoadingMore] = useState(false);
  const [cancelingId, setCancelingId] = useState<string | null>(null);
  const { data, loading, error, startPolling, stopPolling } = useQuery(
    CronJobRunsDocument,
    {
      variables: { serviceId, limit: PAGE_SIZE },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      skipPollAttempt: skipPollWhenHidden,
    },
  );
  const [cancelRun] = useMutation(CancelCronJobRunDocument);

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

  if (
    hasFetched &&
    (retained.serviceId !== serviceId || retained.rows !== fetched)
  ) {
    setRetained({ serviceId, rows: fetched });
  }

  const firstPage = useMemo(
    () =>
      hasFetched
        ? fetched
        : retained.serviceId === serviceId
          ? retained.rows
          : [],
    [hasFetched, fetched, retained, serviceId],
  );

  // Tagged with the service and the head boundary (w4/138, w4/230).
  const {
    rows: runs,
    hasMore,
    next,
  } = useMemo(
    () => joinAppendedPages(firstPage, appendedState, serviceId, PAGE_SIZE),
    [appendedState, serviceId, firstPage],
  );

  const hasActiveRun = runs.some((run) =>
    ACTIVE_STATUSES.has(run.status.toLowerCase()),
  );

  // Fast while a run is in flight, baseline otherwise — the baseline tick is
  // what discovers a scheduled run in a settled history.
  useConvergingPoll(startPolling, stopPolling, hasActiveRun);

  const loadMore = useCallback(async () => {
    if (loadingMore || !next) return;
    setLoadingMore(true);
    try {
      const result = await client.query({
        query: CronJobRunsDocument,
        variables: { serviceId, cursor: next.cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      });
      // An unreadable page is a failure, never a successful empty one that
      // would claim the history is exhausted.
      if (!result.data?.cronJobRuns) {
        throw result.error ?? new Error("no cron runs page");
      }
      const page = toRuns(result.data.cronJobRuns);
      setAppendedState((prev) => appendPage(prev, next, page));
    } catch {
      toast.error(t("services.cronRunsLoadError"));
    } finally {
      setLoadingMore(false);
    }
  }, [client, loadingMore, next, serviceId, t]);

  const cancel = useCallback(
    async (runId: string) => {
      setCancelingId(runId);
      try {
        // The answer is the canceled run, which rewrites its row in the cache;
        // like Render's, a cancel that lands says nothing more.
        await cancelRun({ variables: { serviceId, runId } });
        return true;
      } catch (err) {
        // Render keeps the failure up until it is dismissed.
        toast.error(
          mutationErrorMessage(err, t("services.cronRunCancelError")),
          { duration: Infinity, closeButton: true },
        );
        return false;
      } finally {
        setCancelingId(null);
      }
    },
    [cancelRun, serviceId, t],
  );

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
  };
}

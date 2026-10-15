import { useCallback, useMemo, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelCronJobRunDocument,
  CronJobRunsDocument,
  RunCronJobDocument,
  type CronJobRunsQuery,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { skipPollWhenHidden, useConvergingPoll } from "@/common/lib/polling";
import {
  NO_APPENDED,
  appendPage,
  joinAppendedPages,
  type AppendedPages,
} from "@/common/lib/appended-pages";
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
    useState<AppendedPages<CronRunView>>(NO_APPENDED);
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
      // Inline rather than a toast; a known refusal reads as its localized
      // reason, never the server's English text naming an internal id (w4/203).
      setTriggerError(
        hasGraphQLErrorCode(e, "CRON_SUSPENDED")
          ? t("capabilities.blockedSuspended")
          : mutationErrorMessage(e, t("services.cronTriggerError")),
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

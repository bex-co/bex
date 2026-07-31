import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelCronJobRunDocument,
  CronJobRunsDocument,
  RunCronJobDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
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

/** Cursor-paged cron history + cancellation over the dedicated run-object API. */
export function useCronRuns(serviceId: string): UseCronRunsResult {
  const { t } = useTranslations();
  const loadedMore = useRef(false);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [cancelingId, setCancelingId] = useState<string | null>(null);
  const { data, loading, error, fetchMore, refetch } = useQuery(
    CronJobRunsDocument,
    {
      variables: { serviceId, limit: PAGE_SIZE },
      fetchPolicy: "cache-and-network",
    },
  );
  const [cancelRun] = useMutation(CancelCronJobRunDocument);
  const [runCronJob] = useMutation(RunCronJobDocument);
  const [triggering, setTriggering] = useState(false);
  const [triggerError, setTriggerError] = useState<string | null>(null);

  const runs: CronRunView[] = (data?.cronJobRuns ?? [])
    .filter((run): run is NonNullable<typeof run> => run != null && !!run.id)
    .map((run) => ({
      id: run.id ?? "",
      startedAt: run.startedAt ?? null,
      finishedAt: run.finishedAt ?? null,
      status: run.status ?? "pending",
    }));

  useEffect(() => {
    loadedMore.current = false;
    setHasMore(true);
  }, [serviceId]);

  useEffect(() => {
    if (!loading && !loadedMore.current) {
      setHasMore(runs.length === PAGE_SIZE);
    }
  }, [loading, runs.length]);

  const loadMore = useCallback(async () => {
    const cursor = runs.at(-1)?.id;
    if (!cursor || loadingMore || !hasMore) return;
    loadedMore.current = true;
    setLoadingMore(true);
    try {
      let fetchedCount = 0;
      await fetchMore({
        variables: { serviceId, cursor, limit: PAGE_SIZE },
        updateQuery: (previous, { fetchMoreResult }) => {
          fetchedCount = fetchMoreResult.cronJobRuns?.length ?? 0;
          return {
            cronJobRuns: [
              ...(previous.cronJobRuns ?? []),
              ...(fetchMoreResult.cronJobRuns ?? []),
            ],
          };
        },
      });
      setHasMore(fetchedCount === PAGE_SIZE);
    } catch {
      toast.error(t("services.cronRunsLoadError"));
    } finally {
      setLoadingMore(false);
    }
  }, [fetchMore, hasMore, loadingMore, runs, serviceId, t]);

  const cancel = useCallback(
    async (runId: string) => {
      setCancelingId(runId);
      try {
        await cancelRun({ variables: { serviceId, runId } });
        toast.success(t("services.cronRunCancelSuccess"));
        return true;
      } catch {
        toast.error(t("services.cronRunCancelError"));
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

  const hasActiveRun = runs.some((run) =>
    ACTIVE_STATUSES.has(run.status.toLowerCase()),
  );

  return {
    runs,
    loading,
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

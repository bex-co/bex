import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CancelCronJobRunDocument,
  CronJobRunsDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { CronRunView } from "@/features/services/types";

const PAGE_SIZE = 5;

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

/** Cursor-paged cron history + cancellation over the dedicated run-object API. */
export function useCronRuns(serviceId: string): UseCronRunsResult {
  const { t } = useTranslations();
  const loadedMore = useRef(false);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [cancelingId, setCancelingId] = useState<string | null>(null);
  const { data, loading, error, fetchMore } = useQuery(CronJobRunsDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
  });
  const [cancelRun] = useMutation(CancelCronJobRunDocument);

  const runs: CronRunView[] = (data?.cronJobRuns ?? [])
    .filter((run): run is NonNullable<typeof run> => run != null && !!run.id)
    .map((run) => ({
      id: run.id ?? "",
      name: run.id ?? "",
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

  return {
    runs,
    loading,
    error: !!error,
    loadingMore,
    hasMore,
    cancelingId,
    loadMore,
    cancel,
  };
}

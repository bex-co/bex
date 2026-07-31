import { useQuery } from "@apollo/client/react";
import { CronJobRunDocument } from "@/graphql/definitions";
import type { CronRunView } from "@/features/services/types";

export interface UseCronRunResult {
  run: CronRunView | null;
  loading: boolean;
  /** True when the read errored — e.g. a stale/unknown run id (w5/m60). */
  error: boolean;
}

/**
 * A single cron run's detail via the dedicated `cronJobRun(serviceId, runId)`
 * read (w5/m60) — a fresh status + timing fetch for an expanded history row,
 * skipped until a run is selected. An unknown run id resolves to error, which
 * the panel renders as an explicit state rather than a blank detail.
 */
export function useCronRun(
  serviceId: string,
  runId: string | null,
): UseCronRunResult {
  const { data, loading, error } = useQuery(CronJobRunDocument, {
    variables: { serviceId, runId: runId ?? "" },
    skip: !runId,
    fetchPolicy: "cache-and-network",
  });
  const raw = data?.cronJobRun;
  const run: CronRunView | null =
    raw && raw.id
      ? {
          id: raw.id,
          status: raw.status ?? "pending",
          startedAt: raw.startedAt ?? null,
          finishedAt: raw.finishedAt ?? null,
        }
      : null;
  return { run, loading, error: !!error };
}

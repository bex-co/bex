export type CronRunSummary = {
  id: string;
  status: string;
  startedAt: string | null;
  finishedAt: string | null;
};

const ACTIVE_STATUSES = new Set(["pending", "running"]);
const TERMINAL_STATUSES = new Set([
  "canceled",
  "failed",
  "successful",
  "succeeded",
  "unsuccessful",
]);

export function isActiveCronRun(status: string): boolean {
  return ACTIVE_STATUSES.has(status.trim().toLowerCase());
}

export function isTerminalCronRun(status: string): boolean {
  return TERMINAL_STATUSES.has(status.trim().toLowerCase());
}

export function currentCronRun(
  runs: readonly CronRunSummary[],
): CronRunSummary | null {
  return runs.find((run) => isActiveCronRun(run.status)) ?? null;
}

export function mergeCronRuns(
  current: readonly CronRunSummary[],
  page: readonly CronRunSummary[],
): CronRunSummary[] {
  const result = current.filter((run) => Boolean(run.id));
  const indexById = new Map(result.map((run, index) => [run.id, index]));
  for (const run of page) {
    if (!run.id) continue;
    const index = indexById.get(run.id);
    if (index === undefined) {
      indexById.set(run.id, result.length);
      result.push(run);
    } else {
      result[index] = run;
    }
  }
  return result;
}

/** Composes newest-first pages while giving the refreshed head precedence. */
export function composeCronRunPages(
  head: readonly CronRunSummary[],
  tail: readonly CronRunSummary[],
): CronRunSummary[] {
  const headIds = new Set(head.map((run) => run.id));
  return [...head, ...tail.filter((run) => !headIds.has(run.id))];
}

export function cronRunDuration(run: CronRunSummary): string | null {
  if (!run.startedAt || !run.finishedAt) return null;
  const milliseconds =
    new Date(run.finishedAt).getTime() - new Date(run.startedAt).getTime();
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return null;
  const seconds = Math.round(milliseconds / 1_000);
  if (seconds < 60) return `${seconds}s`;
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

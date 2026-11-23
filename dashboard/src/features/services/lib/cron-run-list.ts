import { makeCustomRange } from "@/features/metrics/lib/range";
import { logRangeToSearch } from "@/features/logs/lib/log-range";
import type { DeployStatusKind } from "@/features/deploys/lib/deploy-status";
import type { CronRunView } from "@/features/services/types";

/**
 * The cron Runs page's list, as Render's dashboard draws it (w3/m556,
 * docs/render-artifacts/cron-dashboard.md § The Runs page): what a run's row
 * says, where it links, and which runs a search keeps.
 */

/** Runs per read: Render's page lists the newest 30. bex keeps every run
 *  since its facts began (w3/m366), so Load more reads 30 older ones. */
export const CRON_RUNS_PAGE_SIZE = 30;

/** Render's four run statuses (its `cronJobRun.status`). bex's older
 *  spellings fold in: a run is in progress until it ends. */
export type CronRunStatus =
  "successful" | "unsuccessful" | "canceled" | "pending";

export function cronRunStatus(run: Pick<CronRunView, "status">): CronRunStatus {
  switch (run.status.toLowerCase()) {
    case "successful":
    case "succeeded":
      return "successful";
    case "unsuccessful":
    case "failed":
      return "unsuccessful";
    case "canceled":
      return "canceled";
    default:
      return "pending";
  }
}

/** The newest successful run among the listed ones (newest first): Render
 *  marks it Last successful run, with the filled check. */
export function latestSuccessfulRunId(runs: CronRunView[]): string | null {
  return runs.find((run) => cronRunStatus(run) === "successful")?.id ?? null;
}

/** The status icon's kind: the deploy list's icons, which Render's run list
 *  shares (a spinner for a run in progress, started or not). */
export function cronRunIconKind(
  status: CronRunStatus,
  latestSuccessful: boolean,
): DeployStatusKind {
  switch (status) {
    case "successful":
      return latestSuccessful ? "live" : "succeeded";
    case "unsuccessful":
      return "failed";
    case "canceled":
      return "canceled";
    default:
      return "in_progress";
  }
}

const MINUTE_MS = 60_000;

/**
 * The Logs over a run's window, Render's `logs?r=<start-1m>~<end+1m>`: from a
 * minute before it started to a minute after it ended, or after `now` while it
 * runs. A run that never started has no start to pad, so the window is the
 * hour before its end, as on Render.
 */
export function cronRunLogsSearch(
  run: Pick<CronRunView, "startedAt" | "finishedAt">,
  now: number,
) {
  const end = parseInstant(run.finishedAt) ?? now;
  const started = parseInstant(run.startedAt);
  const start = started === null ? end - 60 * MINUTE_MS : started - MINUTE_MS;
  const range = makeCustomRange(
    new Date(start).toISOString(),
    new Date(end + MINUTE_MS).toISOString(),
  );
  return typeof range === "string" ? {} : logRangeToSearch(range);
}

/**
 * A run's elapsed milliseconds: start to end, or to `now` in whole seconds
 * while it runs (Render's live duration ticks by the second). Null before it
 * starts.
 */
export function cronRunElapsedMs(
  run: Pick<CronRunView, "startedAt" | "finishedAt">,
  now: number,
): number | null {
  const started = parseInstant(run.startedAt);
  if (started === null) return null;
  const finished = parseInstant(run.finishedAt);
  if (finished !== null) return Math.max(0, finished - started);
  return Math.max(0, Math.floor((now - started) / 1000) * 1000);
}

/** The status words Render's search matches beside the status's name
 *  ("Run failed failed"): its API's own vocabulary. */
export const CRON_RUN_STATUS_WORD: Record<CronRunStatus, string> = {
  pending: "in progress",
  successful: "successful",
  unsuccessful: "failed",
  canceled: "canceled",
};

// bex's run ids are `crr-` and an xid, 24 characters like Render's `run-…`.
const RUN_ID_PREFIX = "crr-";
const RUN_ID_LENGTH = 24;

/**
 * Render's run search (`Search cron job runs`), over the runs the page has
 * loaded. Whitespace and slashes separate the query's words, and a run
 * matches when each word appears in one of its fields: the status as the
 * page names it, the trigger, the triggering user's email, and Last
 * successful run on the newest success. A whole run id is matched against
 * the run's id, as Render matches a `run-…` id. Render ranks with
 * fuzzysort; bex matches words exactly, as its Deploys search does.
 */
export function cronRunMatchesSearch(
  run: Pick<CronRunView, "id">,
  fields: readonly (string | null | undefined)[],
  query: string,
): boolean {
  const normalized = query
    .replace(/[\s/]+/g, " ")
    .trim()
    .toLocaleLowerCase();
  if (normalized === "") return true;
  const haystack = fields.filter(Boolean).join("\n").toLocaleLowerCase();
  const id = run.id.toLocaleLowerCase();
  if (
    normalized.startsWith(RUN_ID_PREFIX) &&
    normalized.length >= RUN_ID_LENGTH &&
    id.includes(normalized)
  ) {
    return true;
  }
  return normalized.split(" ").every((word) => haystack.includes(word));
}

function parseInstant(iso: string | null): number | null {
  if (!iso) return null;
  const value = Date.parse(iso);
  return Number.isNaN(value) ? null : value;
}

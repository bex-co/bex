/**
 * Presentation helpers shared by deploy-history rows and the deploy-detail
 * header. Keeping timestamp and duration parsing here prevents the two
 * surfaces from disagreeing when a timestamp is absent or malformed.
 */

import { formatDateTime } from "@/common/lib/format";

/**
 * Shared so the deploy-history rows and the deploy-detail header render the same
 * text. Inherits `formatDateTime`'s runtime-timezone caveat: only call it behind
 * a hydration gate (`useIsHydrated`) so the UTC SSR clock is never frozen on
 * screen (w6/m107).
 */
export function formatDeployTimestamp(iso: string | null): string | null {
  return formatDateTime(iso);
}

/**
 * The deploy row's subtitle timestamp (w6/051): the verb follows the deploy's
 * terminal state instead of stamping every row "Deployed", and a finished
 * deploy (shipped, failed, or canceled) is stamped with its finish time —
 * falling back to createdAt when no finish time was stored. Rows that haven't
 * finished (created/queued/in-progress, or an unrecognized status) show their
 * creation time under a "Created" verb.
 */
export function deployRowTimestamp(deploy: {
  status: string;
  createdAt: string | null;
  finishedAt: string | null;
}): { key: string; iso: string | null } {
  const finished = deploy.finishedAt ?? deploy.createdAt;
  switch (deploy.status) {
    // A deactivated deploy is a former live one — its finish time is when it
    // went live, so it keeps the "Deployed" verb like Render's history rows.
    case "live":
    case "deactivated":
      return { key: "deploys.deployedAt", iso: finished };
    case "canceled":
      return { key: "deploys.canceledAt", iso: finished };
    case "build_failed":
    case "pre_deploy_failed":
    case "update_failed":
      return { key: "deploys.failedAt", iso: finished };
    default:
      return { key: "deploys.createdAt", iso: deploy.createdAt };
  }
}

export function formatDeployDuration(
  startedAt: string | null,
  finishedAt: string | null,
): string | null {
  const started = parseTimestamp(startedAt);
  const finished = parseTimestamp(finishedAt);
  if (started === null || finished === null || finished < started) return null;

  const elapsedMilliseconds = finished - started;
  const totalSeconds =
    elapsedMilliseconds > 0
      ? Math.max(1, Math.floor(elapsedMilliseconds / 1000))
      : 0;
  if (totalSeconds < 60) return `${totalSeconds}s`;

  const totalMinutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (totalMinutes < 60) {
    return seconds === 0 ? `${totalMinutes}m` : `${totalMinutes}m ${seconds}s`;
  }

  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes === 0 ? `${hours}h` : `${hours}h ${minutes}m`;
}

export function deployMatchesSearch(
  deploy: { id: string; commitId: string; commitMessage: string },
  search: string,
): boolean {
  const needle = search.trim().toLocaleLowerCase();
  if (!needle) return true;
  return [deploy.id, deploy.commitId, deploy.commitMessage].some((value) =>
    value.toLocaleLowerCase().includes(needle),
  );
}

function parseTimestamp(iso: string | null): number | null {
  if (!iso) return null;
  const timestamp = Date.parse(iso);
  return Number.isNaN(timestamp) ? null : timestamp;
}

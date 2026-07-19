/**
 * Presentation helpers shared by deploy-history rows and the deploy-detail
 * header. Keeping timestamp and duration parsing here prevents the two
 * surfaces from disagreeing when a timestamp is absent or malformed.
 */

import { formatDateTime } from "@/common/lib/format";

export function formatDeployTimestamp(iso: string | null): string | null {
  return formatDateTime(iso);
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

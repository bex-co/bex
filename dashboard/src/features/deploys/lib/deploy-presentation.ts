/**
 * Presentation helpers shared by deploy-history rows and the deploy-detail
 * header. Keeping row timestamp selection and duration parsing here prevents
 * the two surfaces from disagreeing when a timestamp is absent or malformed.
 */

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

/**
 * `<short-sha> <subject>` for a deploy's commit — the exact identity the
 * history row renders, so the rollback confirm dialog can name the code it is
 * about to restore in the same words the row behind it uses (w4/m110 t003).
 * Empty when the deploy has no resolvable commit (the `w9/001` contract: an
 * image-backed or unresolvable-repo deploy), which keeps the caller on its
 * generic copy instead of naming nothing.
 */
export function deployCommitLabel(
  commitId: string | null | undefined,
  commitMessage: string | null | undefined,
): string {
  const sha = (commitId ?? "").trim();
  if (!sha) return "";
  const subject = (commitMessage ?? "").split("\n")[0].trim();
  const short = sha.slice(0, 7);
  return subject ? `${short} ${subject}` : short;
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

// Display helpers for a build-from-git App's `spec.repo` (a clone URL), used by
// the service-detail header's source row — Render shows the same thing as
// "<owner> / <repo>" linking to the branch tree, not the raw clone URL.

/**
 * "https://github.com/org/repo.git" -> "org / repo". Falls back to the raw
 * string for anything that isn't a recognizable owner/name pair (a bare path, a
 * self-hosted URL with a deeper path), so the header never hides the source.
 */
export function formatRepoLabel(repo: string): string {
  const path = repoPath(repo);
  if (!path) return repo;
  const parts = path.split("/");
  if (parts.length !== 2) return repo;
  return `${parts[0]} / ${parts[1]}`;
}

/**
 * The browsable URL for `repo` at `branch` (GitHub/GitLab tree layout), or null
 * when the repo isn't an http(s)/scp-style URL we can turn into one — an SSH
 * clone URL still yields a link, since `git@host:org/repo.git` maps cleanly onto
 * `https://host/org/repo`.
 */
export function repoBrowseUrl(
  repo: string,
  branch: string | null,
): string | null {
  const host = repoHost(repo);
  const path = repoPath(repo);
  if (!host || !path) return null;
  const base = `https://${host}/${path}`;
  return branch ? `${base}/tree/${branch}` : base;
}

/** Host of a clone URL — "github.com" for both https:// and git@ spellings. */
function repoHost(repo: string): string | null {
  const https = /^https?:\/\/([^/]+)\//i.exec(repo);
  if (https) return https[1];
  const ssh = /^(?:ssh:\/\/)?git@([^:/]+)[:/]/i.exec(repo);
  return ssh ? ssh[1] : null;
}

/** Path of a clone URL, without a leading slash or the trailing ".git". */
function repoPath(repo: string): string | null {
  const https = /^https?:\/\/[^/]+\/(.+?)(?:\.git)?\/?$/i.exec(repo);
  if (https) return https[1];
  const ssh = /^(?:ssh:\/\/)?git@[^:/]+[:/](.+?)(?:\.git)?\/?$/i.exec(repo);
  return ssh ? ssh[1] : null;
}

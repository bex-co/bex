// Display helpers for a build-from-git App's `spec.repo` (a clone URL), used by
// the service-detail header's source row. The hostname stays visible because
// bex supports self-hosted forges; hiding it behind an owner/repo-only label
// would turn an attacker-controlled workspace repo into a trusted-looking link.

/**
 * "https://github.com/org/repo.git" -> "github.com · org / repo". Falls back to the raw
 * string for anything that isn't a recognizable owner/name pair (a bare path, a
 * self-hosted URL with a deeper path), so the header never hides the source.
 */
export function formatRepoLabel(repo: string): string {
  const host = repoHost(repo);
  const path = repoPath(repo);
  if (!host || !path) return repo;
  const parts = path.split("/");
  if (parts.length !== 2) return repo;
  return `${host} · ${parts[0]} / ${parts[1]}`;
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

/**
 * "owner/name" of a clone URL, without its host. Only for a repository whose
 * forge the page already names (a row picked on the GitHub tab): anywhere
 * else, use formatRepoLabel, which keeps the host in view.
 */
export function repoSlug(repo: string): string | null {
  return repoPath(repo);
}

/**
 * Whether two clone URLs name one repository: the same host and path, however
 * each is spelled (https or ssh, a trailing ".git", letter case, which the
 * forges ignore). An unrecognizable URL is the same as nothing.
 */
export function sameRepo(a: string, b: string): boolean {
  const key = repoKey(a);
  return key !== null && key === repoKey(b);
}

function repoKey(repo: string): string | null {
  const host = repoHost(repo);
  const path = repoPath(repo);
  return host && path ? `${host}/${path}`.toLowerCase() : null;
}

/** Host of a clone URL — "github.com" for both https:// and git@ spellings. */
function repoHost(repo: string): string | null {
  const http = httpRepo(repo);
  if (http) return http.host;
  const ssh = /^(?:ssh:\/\/(?:git@)?|git@)([^:/]+)[:/]/i.exec(repo);
  return ssh ? ssh[1] : null;
}

/** Path of a clone URL, without a leading slash or the trailing ".git". */
function repoPath(repo: string): string | null {
  const http = httpRepo(repo);
  if (http)
    return http.pathname.replace(/^\//, "").replace(/(?:\.git)?\/?$/, "");
  const ssh = /^(?:ssh:\/\/(?:git@)?|git@)[^:/]+[:/](.+?)(?:\.git)?\/?$/i.exec(repo);
  return ssh ? ssh[1] : null;
}

function httpRepo(repo: string): URL | null {
  try {
    const parsed = new URL(repo);
    return parsed.protocol === "http:" || parsed.protocol === "https:"
      ? parsed
      : null;
  } catch {
    return null;
  }
}

/**
 * The web page for one commit of `repo` — "https://host/org/repo/commit/<sha>",
 * where the diff lives — or null when the repo isn't a browsable URL or `sha`
 * isn't a hex commit id (nothing else is ever spliced into the path). GitHub
 * and Gitea serve it at that path and GitLab redirects it to `/-/commit/`, so
 * one layout covers the forges `repoBrowseUrl` already assumes.
 */
export function repoCommitUrl(repo: string, sha: string): string | null {
  if (!/^[0-9a-f]{7,64}$/i.test(sha)) return null;
  const base = repoBrowseUrl(repo, null);
  return base ? `${base}/commit/${sha}` : null;
}

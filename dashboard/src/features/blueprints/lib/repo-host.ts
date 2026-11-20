/**
 * The Git hosts other than github.com that bex reads a Blueprint from: a
 * public repository, anonymously, as Render does (w8/344). bex-api refuses
 * every other host by name.
 */
const PUBLIC_BLUEPRINT_HOSTS = new Set(["gitlab.com", "bitbucket.org"]);

/**
 * Where a Blueprint repository lives, for the review's failure copy (w8/346):
 * `github` for a repository bex reads through its GitHub app, `public` for a
 * gitlab.com or bitbucket.org one, `unsupported` for any other host. `host`
 * is the lowercased host the URL names.
 */
export type BlueprintRepoHost =
  { kind: "github" } | { kind: "public" | "unsupported"; host: string };

const SCP_HOST_RE = /^[^@\s/]+@([^:/\s]+):/;

/**
 * The host of a Blueprint repository URL, as bex-api's Blueprint read picks
 * it: an `https://` or `ssh://` URL's hostname, or a `git@host:owner/repo`
 * spelling's host. A URL that names no host reads as GitHub's, the copy the
 * page showed before.
 */
export function blueprintRepoHost(repo: string): BlueprintRepoHost {
  const value = repo.trim();
  let host = SCP_HOST_RE.exec(value)?.[1] ?? "";
  if (!host) {
    try {
      host = new URL(value).hostname;
    } catch {
      host = "";
    }
  }
  host = host.toLowerCase();
  if (host === "" || host === "github.com") return { kind: "github" };
  return {
    kind: PUBLIC_BLUEPRINT_HOSTS.has(host) ? "public" : "unsupported",
    host,
  };
}

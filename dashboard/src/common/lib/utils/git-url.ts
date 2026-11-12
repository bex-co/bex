// A clone URL the backend will accept (bex-api's store.RepoRule): https://, or
// one of GitHub's SSH spellings, which the build fetches over HTTPS (w8/130).
// The prefix is all the client checks, since the real validation
// (reachability, auth) only happens when the build actually clones. Shared by
// every create form that takes a public repo URL, so the two of them can't
// drift into accepting different things.
const GIT_URL_PREFIX_RE =
  /^(https:\/\/|git@github\.com:|ssh:\/\/(git@)?github\.com\/)/;

export function isValidGitUrl(url: string): boolean {
  return GIT_URL_PREFIX_RE.test(url.trim());
}

/** A GitHub folder (`tree`) or file (`blob`) page split into its parts. */
export interface GitHubPage {
  repo: string;
  branch: string;
  rootDir: string;
  file: boolean;
}

const GITHUB_PAGE_RE =
  /^https:\/\/(?:www\.)?github\.com\/([^/]+)\/([^/]+?)(?:\.git)?\/(tree|blob)\/([^/?#]+)(?:\/([^?#]*?))?\/?(?:[?#].*)?$/i;

/**
 * The repository, branch and folder of a GitHub folder or file page, the URL a
 * browser shows, which no clone accepts (w4/576); null for any other URL. A
 * branch with a `/` reads as branch plus folder, which the fields it fills
 * leave editable.
 */
export function parseGitHubPage(url: string): GitHubPage | null {
  const m = GITHUB_PAGE_RE.exec(url.trim());
  if (!m) return null;
  return {
    repo: `https://github.com/${m[1]}/${m[2]}`,
    branch: decodePathPart(m[4]),
    rootDir: decodePathPart(m[5] ?? ""),
    file: m[3].toLowerCase() === "blob",
  };
}

function decodePathPart(part: string): string {
  try {
    return decodeURIComponent(part);
  } catch {
    return part;
  }
}

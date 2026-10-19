// A clone URL the backend will accept (bex-api's store.RepoRule): https://, or
// one of GitHub's SSH spellings, which the build fetches over HTTPS (w8/130).
// The prefix is all the client checks, since the real validation
// (reachability, auth) only happens when the build actually clones. Shared by
// every create form that takes a public repo URL, so the two of them can't
// drift into accepting different things.
const GIT_URL_PREFIX_RE = /^(https:\/\/|git@github\.com:|ssh:\/\/(git@)?github\.com\/)/;

export function isValidGitUrl(url: string): boolean {
  return GIT_URL_PREFIX_RE.test(url.trim());
}

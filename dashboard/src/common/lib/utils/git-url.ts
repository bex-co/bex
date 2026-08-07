// A clone URL the backend will accept: the scheme prefix is all the client
// checks, since the real validation (reachability, auth) only happens when the
// build actually clones. Shared by every create form that takes a public repo
// URL, so the two of them can't drift into accepting different things.
const GIT_URL_SCHEME_RE = /^(https?:\/\/|git@|git:\/\/)/;

export function isValidGitUrl(url: string): boolean {
  return GIT_URL_SCHEME_RE.test(url.trim());
}

/**
 * The backend answers ErrGitHubUnavailable (503) when BEX_GITHUB_APP_* is unset;
 * that message flows through GraphQL as "github integration not configured". Both
 * the Settings ConnectGithubCard and the in-place GitCredentialsMenu (w8/m31)
 * classify that state, so the predicate lives here once rather than drifting
 * between two copies.
 */
export function isGitHubUnavailable(error: Error | undefined): boolean {
  if (!error) return false;
  const m = error.message.toLowerCase();
  return m.includes("not configured") || m.includes("unavailable");
}

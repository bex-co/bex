import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";

/**
 * The backend answers ErrGitHubUnavailable (503, GITHUB_UNAVAILABLE) when
 * BEX_GITHUB_APP_* is unset. Both the Settings ConnectGithubCard and the
 * in-place GitCredentialsMenu (w8/m31) classify that state, so the predicate
 * lives here once rather than drifting between two copies; it reads the code,
 * not the wording (w5/m130).
 */
export function isGitHubUnavailable(error: Error | undefined): boolean {
  return hasGraphQLErrorCode(error, "GITHUB_UNAVAILABLE");
}

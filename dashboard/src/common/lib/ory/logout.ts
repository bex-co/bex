import type { FrontendApi } from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { getClient } from "@/common/apollo/client";

/**
 * True when Kratos refuses the logout flow because there is already no
 * session (unsigned visitor clicked Sign out). A 401/403 here is not a
 * provider outage — the cookie is already gone — so callers should treat it
 * as success rather than the red "you may still be signed in" error.
 */
function isAlreadySignedOut(err: unknown): boolean {
  const response = (err as { response?: Response })?.response;
  if (response) {
    return response.status === 401 || response.status === 403;
  }
  return false;
}

export async function clearBrowserAccountState(): Promise<void> {
  // The CSR Apollo client is a module singleton that survives logout, so
  // without this the next account could read the previous one's cached
  // workspaces/resources (codex-security #24).
  invalidateSessionCache();
  await getClient().clearStore();
}

/**
 * Ask Kratos whether the session still exists. The logout fetch's own result
 * is not trustworthy (see `endBrowserSession`), so ground truth is `whoami`:
 * 401/403 means the session is gone — logout actually succeeded.
 */
async function sessionIsGone(api: FrontendApi): Promise<boolean> {
  try {
    await api.toSession();
    return false;
  } catch (err) {
    return isAlreadySignedOut(err);
  }
}

/**
 * End the browser's Kratos session and drop locally cached account-scoped
 * data. Does not navigate — callers decide where to send the user.
 *
 * Treats the flow as done when Kratos returns a successful response
 * (codex #6), OR when Kratos reports there is already no session (401/403
 * on createBrowserLogoutFlow / the logout URL). Presenting failure while
 * the cookie is already gone was the live `/auth/logout` bug. Real
 * provider failures (5xx, network) still throw so callers keep the retry UI.
 *
 * The logout GET's own response can't be trusted either: Kratos deletes the
 * session and then 303-redirects to `return_to` (the dashboard origin), and
 * fetch's CORS mode rejects that redirect because the dashboard doesn't send
 * Access-Control-Allow-Origin — so a *successful* logout surfaced as a
 * network error on the first attempt, and the retry "worked" only because
 * createBrowserLogoutFlow then 401'd. Any failure of the logout fetch is
 * therefore confirmed against `whoami` before reporting an error.
 */
export async function endBrowserSession(): Promise<void> {
  const api = createFrontendApi();
  let logoutUrl: string;
  try {
    const flow = await api.createBrowserLogoutFlow({
      returnTo: `${window.location.origin}/`,
    });
    logoutUrl = flow.logout_url;
  } catch (err) {
    if (isAlreadySignedOut(err)) {
      await clearBrowserAccountState();
      return;
    }
    throw err;
  }

  try {
    const response = await fetch(logoutUrl, { credentials: "include" });
    if (!response.ok && response.status !== 401 && response.status !== 403) {
      throw new Error(`logout request failed: ${response.status}`);
    }
  } catch (err) {
    if (!(await sessionIsGone(api))) {
      throw err;
    }
  }

  await clearBrowserAccountState();
}

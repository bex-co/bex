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

async function clearLocalSessionState(): Promise<void> {
  // The CSR Apollo client is a module singleton that survives logout, so
  // without this the next account could read the previous one's cached
  // workspaces/resources (codex-security #24).
  invalidateSessionCache();
  await getClient().clearStore();
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
      await clearLocalSessionState();
      return;
    }
    throw err;
  }

  const response = await fetch(logoutUrl, { credentials: "include" });
  if (!response.ok) {
    if (response.status === 401 || response.status === 403) {
      await clearLocalSessionState();
      return;
    }
    throw new Error(`logout request failed: ${response.status}`);
  }

  await clearLocalSessionState();
}

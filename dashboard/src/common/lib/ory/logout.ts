import { createFrontendApi } from "@/common/lib/ory/frontend";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { getClient } from "@/common/apollo/client";

/**
 * End the browser's Kratos session and drop locally cached account-scoped
 * data. Does not navigate — callers decide where to send the user.
 *
 * Treats the flow as done ONLY when Kratos returns a successful response
 * (codex #6): presenting success while the HttpOnly cookie is still valid
 * would let the next user of this browser inherit the session. Local cache
 * clearing happens only on that success.
 */
export async function endBrowserSession(): Promise<void> {
  const api = createFrontendApi();
  const { logout_url } = await api.createBrowserLogoutFlow();
  const response = await fetch(logout_url, { credentials: "include" });
  // fetch resolves on 4xx/5xx, so an unchecked response would hide a failed
  // logout. Require a successful provider response before treating the
  // session as ended.
  if (!response.ok) {
    throw new Error(`logout request failed: ${response.status}`);
  }

  // The CSR Apollo client is a module singleton that survives logout, so
  // without this the next account could read the previous one's cached
  // workspaces/resources (codex-security #24).
  invalidateSessionCache();
  await getClient().clearStore();
}

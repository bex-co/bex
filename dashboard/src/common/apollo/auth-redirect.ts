import {
  fetchSession,
  invalidateSessionCache,
} from "@/common/server-fn/session";
import { currentHref } from "@/common/lib/safe-next";

/**
 * The login URL that sends an expired session back to sign-in and then back to
 * where it was. `next` is the current location (path + search + hash) so the
 * login page's `safeNext` round-trips the user to the same page; `aal=aal2` is
 * added only when the re-check found a live aal1 session owing a second factor
 * (a step-up, not a sign-in) — the same distinction `requireAuth` makes.
 */
export function buildLoginRedirectHref(
  currentHref: string,
  aal2Required: boolean,
): string {
  const params = new URLSearchParams();
  if (currentHref) params.set("next", currentHref);
  if (aal2Required) params.set("aal", "aal2");
  const query = params.toString();
  return query ? `/auth/login?${query}` : "/auth/login";
}

/**
 * True once a redirect is committed, so the burst of queries a page fires (each
 * of which 401s) doesn't start its own navigation. A real redirect unloads the
 * document, so this never needs resetting in the app — only tests clear it.
 */
let redirecting = false;

/** Test-only: clear the in-flight-redirect guard between cases. */
export function resetAuthRedirectForTests(): void {
  redirecting = false;
}

/**
 * React to a 401 from bex-api on the client (w3/m80 t001). It confirms the
 * session is actually gone before bouncing: a 401 with a still-live Kratos
 * session (clock skew, or a transient auth-upstream blip on bex-api's side)
 * must NOT force a logout, so those fall through and surface as an ordinary
 * error the user can retry. When the session is genuinely gone, hard-navigate
 * to login carrying `next` back to the current page.
 *
 * The hard navigation (not a soft router hop) is deliberate: it clears the
 * singleton Apollo cache of the prior session's data and re-runs SSR
 * unauthenticated, and t005's editor drafts survive it via sessionStorage.
 */
export async function handleUnauthenticated(): Promise<void> {
  if (typeof window === "undefined") return; // client-only; SSR redirects via loaders
  if (redirecting) return;
  // Never bounce login → login: the auth pages make their own requests, and a
  // 401 there is the flow's business, not an expiry.
  if (window.location.pathname.startsWith("/auth/")) return;

  invalidateSessionCache();
  const { session, aal2Required } = await fetchSession();
  if (session) return; // 401 but the session is still live — not an expiry
  if (redirecting) return; // a concurrent 401 already won the race

  redirecting = true;
  window.location.assign(buildLoginRedirectHref(currentHref(), aal2Required));
}

/** Where a not-yet-authenticated visitor's workspace-invite token waits out
 *  the Kratos sign-up/login round-trip (w1/m33): the auth pages stash it, the
 *  authenticated layout's redemption hook (features/team) redeems it. Lives in
 *  common so the auth chrome doesn't import a peer feature. */
export const INVITE_TOKEN_STORAGE_KEY = "bex.pendingInviteToken";

/** Stashes an `?invite=<token>` from the current URL for post-auth redemption
 *  — called by the auth page shell, where the invite email's link lands. Safe
 *  on the server (no-op without a window). */
export function stashInviteTokenFromURL() {
  if (typeof window === "undefined") return;
  const token = new URLSearchParams(window.location.search).get("invite");
  if (token) window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, token);
}

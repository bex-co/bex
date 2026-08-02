/** Where a not-yet-authenticated visitor's workspace-invite bearer capability
 * waits out the Kratos sign-up/login round-trip. sessionStorage keeps it scoped
 * to this tab and out of durable browser storage. */
export const INVITE_TOKEN_STORAGE_KEY = "bex.pendingInviteToken";

const INVITE_TOKEN_PATTERN = /^[a-f0-9]{32}$/;

export function validInviteToken(value: unknown): value is string {
  return typeof value === "string" && INVITE_TOKEN_PATTERN.test(value);
}

export type InviteCapture = "none" | "stored" | "invalid" | "unavailable";

/**
 * Validate and capture exactly one `invite` query value, then scrub it from
 * browser history before navigation. Duplicate, oversized, uppercase, or
 * otherwise malformed values are terminally rejected and clear any stale
 * pending token. No other query value is ever copied into storage.
 *
 * `scrubAll` is for the dedicated `/invite` handoff: it replaces the complete
 * query/hash with the bare pathname. Auth pages retain their unrelated Ory
 * flow parameters while deleting only `invite`.
 */
export function stashInviteTokenFromURL(options?: {
  scrubAll?: boolean;
}): InviteCapture {
  if (typeof window === "undefined") return "none";

  const url = new URL(window.location.href);
  const values = url.searchParams.getAll("invite");
  if (values.length === 0) return "none";

  const token = values.length === 1 ? values[0] : null;
  let result: InviteCapture = "invalid";
  try {
    if (validInviteToken(token)) {
      window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, token);
      result = "stored";
    } else {
      window.sessionStorage.removeItem(INVITE_TOKEN_STORAGE_KEY);
    }
  } catch {
    // Storage denial must not leave a bearer capability in the address bar.
    result = "unavailable";
  }

  if (options?.scrubAll) {
    window.history.replaceState(window.history.state, "", url.pathname);
  } else {
    url.searchParams.delete("invite");
    window.history.replaceState(
      window.history.state,
      "",
      `${url.pathname}${url.search}${url.hash}`,
    );
  }
  return result;
}

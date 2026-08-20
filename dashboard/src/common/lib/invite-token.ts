/** Where a not-yet-authenticated visitor's workspace-invite bearer capability
 * waits out the Kratos sign-up/login round-trip. sessionStorage keeps it scoped
 * to this tab and out of durable browser storage. */
export const INVITE_TOKEN_STORAGE_KEY = "bex.pendingInviteToken";

const INVITE_TOKEN_PATTERN = /^[a-f0-9]{32}$/;

export function validInviteToken(value: unknown): value is string {
  return typeof value === "string" && INVITE_TOKEN_PATTERN.test(value);
}

/** Return only an exact invite capability; never coerce an arbitrary value. */
export function parseInviteToken(value: unknown): string | null {
  return validInviteToken(value) ? value : null;
}

export type InviteCapture = "none" | "stored" | "invalid" | "unavailable";

function scrubInviteFromURL(url: URL, scrubAll = false) {
  if (scrubAll) {
    window.history.replaceState(window.history.state, "", url.pathname);
    return;
  }

  url.searchParams.delete("invite");
  window.history.replaceState(
    window.history.state,
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
}

function clearPendingInviteToken(): boolean {
  try {
    window.sessionStorage.removeItem(INVITE_TOKEN_STORAGE_KEY);
    return true;
  } catch {
    return false;
  }
}

/**
 * Replace a pending capability without allowing a failed write to expose an
 * older capability to the next authenticated session.
 */
function replacePendingInviteToken(token: string): boolean {
  if (!clearPendingInviteToken()) return false;
  try {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, token);
    return true;
  } catch {
    // Quota/security failures can occur after removeItem succeeded. Clear once
    // more in case a non-standard Storage implementation partially wrote.
    clearPendingInviteToken();
    return false;
  }
}

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

  const token = parseInviteToken(values.length === 1 ? values[0] : null);

  // Remove the bearer from browser history before touching fallible storage.
  scrubInviteFromURL(url, options?.scrubAll);

  if (!token) {
    return clearPendingInviteToken() ? "invalid" : "unavailable";
  }
  return replacePendingInviteToken(token) ? "stored" : "unavailable";
}

/** Peek at a pending capability without consuming it — used to offer explicit
 * acceptance after authenticated navigation (codex round-16 #8). */
export function peekPendingInviteToken(): string | null {
  if (typeof window === "undefined") return null;

  const url = new URL(window.location.href);
  const values = url.searchParams.getAll("invite");
  if (values.length > 0) {
    return parseInviteToken(values.length === 1 ? values[0] : null);
  }

  try {
    return parseInviteToken(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY));
  } catch {
    return null;
  }
}

/**
 * Consume one pending capability for authenticated redemption. A present URL
 * parameter always wins: malformed or duplicate parameters are rejected and
 * may not fall back to an older stored token. The URL is scrubbed synchronously
 * before any storage access.
 */
export function takePendingInviteToken(): string | null {
  if (typeof window === "undefined") return null;

  const url = new URL(window.location.href);
  const values = url.searchParams.getAll("invite");
  if (values.length > 0) {
    scrubInviteFromURL(url);
    if (!clearPendingInviteToken()) return null;
    return parseInviteToken(values.length === 1 ? values[0] : null);
  }

  let stored: string | null;
  try {
    stored = window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY);
  } catch {
    clearPendingInviteToken();
    return null;
  }
  if (!clearPendingInviteToken()) return null;
  return parseInviteToken(stored);
}

/** Retain an exact capability only for an explicit retry after ambiguity. */
export function retainPendingInviteToken(token: unknown): boolean {
  if (typeof window === "undefined") return false;
  const parsed = parseInviteToken(token);
  if (!parsed) {
    clearPendingInviteToken();
    return false;
  }
  return replacePendingInviteToken(parsed);
}

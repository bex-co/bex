/** Where a not-yet-authenticated visitor's invite bearer capability waits out
 * the Kratos sign-up/login round-trip. sessionStorage keeps it scoped to this
 * tab and out of durable browser storage. */
export const INVITE_TOKEN_STORAGE_KEY = "bex.pendingInviteToken";

const INVITE_TOKEN_PATTERN = /^[a-f0-9]{32}$/;

/** A workspace invite's link carries `invite=<token>`; an organization
 * invite's carries `org-invite=<token>` (w7/083). Both redeem the same way. */
export type InviteKind = "workspace" | "organization";

export type PendingInvite = { kind: InviteKind; token: string };

const URL_KEYS: Record<InviteKind, string> = {
  workspace: "invite",
  organization: "org-invite",
};

// A stored organization capability is prefixed so the one storage slot can
// never be redeemed as the other kind. A bare token is a workspace invite.
const ORGANIZATION_STORAGE_PREFIX = "org:";

export function validInviteToken(value: unknown): value is string {
  return typeof value === "string" && INVITE_TOKEN_PATTERN.test(value);
}

/** Return only an exact invite capability; never coerce an arbitrary value. */
export function parseInviteToken(value: unknown): string | null {
  return validInviteToken(value) ? value : null;
}

export type InviteCapture = "none" | "stored" | "invalid" | "unavailable";

function fragmentParams(url: URL): URLSearchParams | null {
  const fragment = url.hash.startsWith("#") ? url.hash.slice(1) : url.hash;
  return fragment ? new URLSearchParams(fragment) : null;
}

function scrubInviteFromURL(url: URL, scrubAll = false) {
  if (scrubAll) {
    window.history.replaceState(window.history.state, "", url.pathname);
    return;
  }

  const fragment = fragmentParams(url);
  for (const key of Object.values(URL_KEYS)) {
    url.searchParams.delete(key);
    if (fragment?.has(key)) {
      fragment.delete(key);
      const nextFragment = fragment.toString();
      url.hash = nextFragment ? `#${nextFragment}` : "";
    }
  }
  window.history.replaceState(
    window.history.state,
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
}

/** Every invite capability the URL carries, of either kind. The dedicated
 * email handoff uses a fragment so the bearer is absent from the initial HTTP
 * request. Keep accepting the historical query shape for already-issued links
 * and auth-flow compatibility, but never write it back into a URL. */
function invitesFromURL(url: URL): { kind: InviteKind; value: string }[] {
  const fragment = fragmentParams(url);
  return (Object.keys(URL_KEYS) as InviteKind[]).flatMap((kind) =>
    [
      ...url.searchParams.getAll(URL_KEYS[kind]),
      ...(fragment?.getAll(URL_KEYS[kind]) ?? []),
    ].map((value) => ({ kind, value })),
  );
}

/** Exactly one well-formed capability, or null: duplicates, a mix of kinds,
 * and malformed values are all rejected. */
function inviteFromURLValues(
  values: { kind: InviteKind; value: string }[],
): PendingInvite | null {
  if (values.length !== 1) return null;
  const token = parseInviteToken(values[0].value);
  return token ? { kind: values[0].kind, token } : null;
}

function encodePendingInvite(invite: PendingInvite): string {
  return invite.kind === "organization"
    ? `${ORGANIZATION_STORAGE_PREFIX}${invite.token}`
    : invite.token;
}

function decodePendingInvite(stored: string | null): PendingInvite | null {
  if (stored?.startsWith(ORGANIZATION_STORAGE_PREFIX)) {
    const token = parseInviteToken(
      stored.slice(ORGANIZATION_STORAGE_PREFIX.length),
    );
    return token ? { kind: "organization", token } : null;
  }
  const token = parseInviteToken(stored);
  return token ? { kind: "workspace", token } : null;
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
function replacePendingInvite(invite: PendingInvite): boolean {
  if (!clearPendingInviteToken()) return false;
  try {
    window.sessionStorage.setItem(
      INVITE_TOKEN_STORAGE_KEY,
      encodePendingInvite(invite),
    );
    return true;
  } catch {
    // Quota/security failures can occur after removeItem succeeded. Clear once
    // more in case a non-standard Storage implementation partially wrote.
    clearPendingInviteToken();
    return false;
  }
}

/**
 * Validate and capture exactly one `invite` or `org-invite` value, then scrub
 * it from browser history before navigation. Duplicate, oversized, uppercase,
 * or otherwise malformed values are terminally rejected and clear any stale
 * pending token. No other query value is ever copied into storage.
 *
 * `scrubAll` is for the dedicated `/invite` handoff: it replaces the complete
 * query/hash with the bare pathname. Auth pages retain their unrelated Ory
 * flow parameters while deleting only the invite keys.
 */
export function stashInviteTokenFromURL(options?: {
  scrubAll?: boolean;
}): InviteCapture {
  if (typeof window === "undefined") return "none";

  const url = new URL(window.location.href);
  const values = invitesFromURL(url);
  if (values.length === 0) return "none";

  const invite = inviteFromURLValues(values);

  // Remove the bearer from browser history before touching fallible storage.
  scrubInviteFromURL(url, options?.scrubAll);

  if (!invite) {
    return clearPendingInviteToken() ? "invalid" : "unavailable";
  }
  return replacePendingInvite(invite) ? "stored" : "unavailable";
}

/** Peek at a pending capability without consuming it — used to offer explicit
 * acceptance after authenticated navigation (codex round-16 #8). */
export function peekPendingInvite(): PendingInvite | null {
  if (typeof window === "undefined") return null;

  const url = new URL(window.location.href);
  const values = invitesFromURL(url);
  if (values.length > 0) return inviteFromURLValues(values);

  try {
    return decodePendingInvite(
      window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY),
    );
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
export function takePendingInvite(): PendingInvite | null {
  if (typeof window === "undefined") return null;

  const url = new URL(window.location.href);
  const values = invitesFromURL(url);
  if (values.length > 0) {
    scrubInviteFromURL(url);
    if (!clearPendingInviteToken()) return null;
    return inviteFromURLValues(values);
  }

  let stored: string | null;
  try {
    stored = window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY);
  } catch {
    clearPendingInviteToken();
    return null;
  }
  if (!clearPendingInviteToken()) return null;
  return decodePendingInvite(stored);
}

/** Retain an exact capability only for an explicit retry after ambiguity. */
export function retainPendingInviteToken(token: unknown): boolean {
  if (typeof window === "undefined") return false;
  const parsed = parseInviteToken(token);
  if (!parsed) {
    clearPendingInviteToken();
    return false;
  }
  return replacePendingInvite({ kind: "workspace", token: parsed });
}

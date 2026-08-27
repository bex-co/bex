import type { Session } from "@ory/client-fetch";
import { createIsomorphicFn } from "@tanstack/react-start";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { oryErrorInfo } from "@/common/lib/ory/errors";

/**
 * The incoming request's Cookie header when rendering on the server; in the
 * browser, fetch sends cookies itself (`credentials: "include"`). An
 * isomorphic fn (not a bare `import.meta.env.SSR` guard) so the compiler
 * strips the server-only import from the client bundle — otherwise
 * TanStack Start's import-protection warns on every dev-server boot.
 */
const getRequestCookie = createIsomorphicFn()
  .server(() =>
    import("@tanstack/react-start/server").then((m) =>
      m.getRequestHeader("cookie"),
    ),
  )
  .client(() => undefined);

/**
 * What Kratos says about the browser behind this request. `session` is null
 * whenever `whoami` refuses — but *why* it refused matters: under the
 * `highest_available` AAL policy (docs/ADR012-auth.md § MFA) someone who signed in
 * with a password but still owes a second factor holds a perfectly live aal1
 * session that `whoami` nonetheless 403s, with `session_aal2_required`. That is a
 * step-up, not a sign-in, and this call is the only place that can tell the two
 * apart — so it says which it is, instead of leaving the login page to
 * rediscover it by minting a trial flow (w4/m17).
 */
export type SessionState = {
  session: Session | null;
  /** A live aal1 session owing its second factor: challenge, don't sign in. */
  aal2Required: boolean;
  /**
   * When the live session expires, as epoch ms (Kratos's `expires_at`), or
   * undefined when there is no session or Kratos omits it. Used to evict a
   * lapsed session from the browser memo the instant it passes, so the app
   * never reports "authenticated" for a session that has actually expired
   * (w3/m80 t004) — the sliding-session config (docs/ADR012-auth.md § Sessions)
   * keeps this moving forward for an active user.
   */
  expiresAt?: number;
};

/**
 * How long a browser-side `whoami` answer is reused before asking Kratos
 * again. The root route's `beforeLoad` calls `fetchSession()` on every
 * navigation *and* every hover-triggered preload (`defaultPreload: "intent"`),
 * so without this memo a mouse pass over the sidebar fires a burst of
 * identical `/sessions/whoami` requests. Auth transitions don't wait out the
 * TTL: login/register/logout call `invalidateSessionCache()` first.
 */
const SESSION_CACHE_TTL_MS = 60_000;

type SessionCacheEntry = {
  at: number;
  state: Promise<SessionState>;
  /** Filled once the promise resolves; drives early eviction on expiry. */
  expiresAt?: number;
};

let sessionCache: SessionCacheEntry | null = null;

/**
 * Drop the memoized browser session so the next `fetchSession()` asks Kratos.
 * Call before `router.invalidate()` whenever the session itself changed
 * (sign-in, sign-up, sign-out) — invalidating the router re-runs the root
 * `beforeLoad`, and that re-run must not be answered from the memo.
 */
export function invalidateSessionCache(): void {
  sessionCache = null;
}

async function fetchSessionUncached(cookie?: string): Promise<SessionState> {
  try {
    const session = await createFrontendApi(
      cookie ?? (await getRequestCookie()),
    ).toSession();
    return {
      session,
      aal2Required: false,
      expiresAt: parseExpiry(session.expires_at),
    };
  } catch (err) {
    const { id } = await oryErrorInfo(err);
    return { session: null, aal2Required: id === "session_aal2_required" };
  }
}

/** Kratos's `expires_at` as epoch ms, or undefined when absent/unparseable. The
 *  Ory SDK types it as a `Date`, but tolerate an ISO string too for safety. */
function parseExpiry(expiresAt: Date | string | undefined): number | undefined {
  if (!expiresAt) return undefined;
  const ms =
    expiresAt instanceof Date ? expiresAt.getTime() : Date.parse(expiresAt);
  return Number.isNaN(ms) ? undefined : ms;
}

/**
 * Whether the memo can answer for `now`: present, within the dedup TTL, and not
 * already past its own `expires_at`. The expiry check is what stops a session
 * that lapsed mid-TTL from being reported "authenticated" for up to a minute —
 * the next read re-consults Kratos, which 401s, and the auth guard redirects.
 */
function isCacheUsable(now: number): boolean {
  if (!sessionCache) return false;
  if (now - sessionCache.at > SESSION_CACHE_TTL_MS) return false;
  if (sessionCache.expiresAt != null && sessionCache.expiresAt <= now) {
    return false;
  }
  return true;
}

/**
 * The Kratos session behind the current request. Pass `cookie` explicitly from a
 * server route handler, which holds the `Request` itself (`hydra-consent.ts`);
 * omit it and the ambient request's Cookie header is used.
 *
 * In the browser the answer is memoized for a short TTL (and concurrent calls
 * share one in-flight request). Never on the server: module state there
 * outlives a request, so a memo would leak one visitor's session into
 * another's render — every SSR call consults Kratos with the request's own
 * Cookie header.
 */
export async function fetchSession(cookie?: string): Promise<SessionState> {
  if (import.meta.env.SSR || cookie !== undefined) {
    return fetchSessionUncached(cookie);
  }
  const now = Date.now();
  if (!isCacheUsable(now)) {
    const entry: SessionCacheEntry = { at: now, state: fetchSessionUncached() };
    // Record the resolved expiry so `isCacheUsable` can evict a lapsed session
    // on the very next read (the fetch never rejects — it resolves a
    // null-session state on failure).
    void entry.state.then((s) => {
      entry.expiresAt = s.expiresAt;
    });
    sessionCache = entry;
  }
  return sessionCache!.state;
}

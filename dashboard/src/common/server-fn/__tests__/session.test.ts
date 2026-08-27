import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchSession, invalidateSessionCache } from "../session";

const toSession = vi.fn();
vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => ({ toSession }),
}));

/** How the SDK surfaces a refused whoami: the raw Response, body and all. */
const refused = (status: number, body: unknown) => ({
  response: new Response(JSON.stringify(body), { status }),
});

describe("fetchSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    invalidateSessionCache();
  });

  it("returns the live session", async () => {
    const session = { id: "session-1", identity: { id: "user-1" } };
    toSession.mockResolvedValue(session);

    expect(await fetchSession("ory_session=live")).toEqual({
      session,
      aal2Required: false,
    });
  });

  it("reports a second factor owed when whoami 403s with session_aal2_required", async () => {
    // The `highest_available` AAL policy (docs/ADR012-auth.md § MFA): a password-only
    // login leaves a live aal1 session that whoami refuses until the second factor
    // is presented. It is NOT "not signed in" — telling the two apart here is what
    // lets the auth guard send the user to a step-up instead of a sign-in form,
    // and it is the only place that can (w4/m17).
    toSession.mockRejectedValue(
      refused(403, { error: { id: "session_aal2_required" } }),
    );

    expect(await fetchSession("ory_session=aal1")).toEqual({
      session: null,
      aal2Required: true,
    });
  });

  it("reports no session when whoami 401s", async () => {
    toSession.mockRejectedValue(
      refused(401, { error: { id: "session_inactive" } }),
    );

    expect(await fetchSession("ory_session=stale")).toEqual({
      session: null,
      aal2Required: false,
    });
  });

  it("reports no session when Kratos is unreachable", async () => {
    // No response to read an error id off — a transport failure is not a step-up.
    toSession.mockRejectedValue(new TypeError("fetch failed"));

    expect(await fetchSession("ory_session=live")).toEqual({
      session: null,
      aal2Required: false,
    });
  });
});

describe("fetchSession browser memo", () => {
  // No `cookie` argument = the browser path (in vitest's jsdom,
  // import.meta.env.SSR is false), where whoami is memoized: the root
  // route's beforeLoad runs on every navigation and hover-preload, and
  // each run must not become its own /sessions/whoami request.
  const session = { id: "session-1", identity: { id: "user-1" } };

  beforeEach(() => {
    vi.clearAllMocks();
    invalidateSessionCache();
    toSession.mockResolvedValue(session);
  });

  afterEach(() => vi.useRealTimers());

  it("answers repeated calls from one whoami request", async () => {
    // Concurrent (hover-preload burst) and sequential (tab navigation)
    // callers alike share the single in-flight/settled request.
    const [a, b] = await Promise.all([fetchSession(), fetchSession()]);
    const c = await fetchSession();

    expect(toSession).toHaveBeenCalledTimes(1);
    expect(a.session).toEqual(session);
    expect(b.session).toEqual(session);
    expect(c.session).toEqual(session);
  });

  it("asks Kratos again once the TTL lapses", async () => {
    vi.useFakeTimers();
    await fetchSession();
    vi.advanceTimersByTime(61_000);
    await fetchSession();

    expect(toSession).toHaveBeenCalledTimes(2);
  });

  it("asks Kratos again after invalidateSessionCache", async () => {
    // The login/register/logout pages call this before router.invalidate():
    // the re-run of the root beforeLoad must see the new session, not the memo.
    await fetchSession();
    invalidateSessionCache();
    await fetchSession();

    expect(toSession).toHaveBeenCalledTimes(2);
  });

  it("never serves the memo to an explicit-cookie caller", async () => {
    // Explicit cookies come from server route handlers (hydra-consent.ts):
    // per-request truth, never shared state.
    await fetchSession();
    await fetchSession("ory_session=other");

    expect(toSession).toHaveBeenCalledTimes(2);
  });
});

// w3/m80 t004: the memo dedups whoami, but it must never keep reporting
// "authenticated" for a session that has actually lapsed within the TTL window
// — the expiry check evicts it early so the next read re-consults Kratos.
describe("fetchSession expiry eviction", () => {
  const base = { id: "session-1", identity: { id: "user-1" } };

  beforeEach(() => {
    vi.clearAllMocks();
    invalidateSessionCache();
  });
  afterEach(() => vi.useRealTimers());

  it("surfaces the session's expires_at as epoch ms", async () => {
    const exp = new Date("2026-01-01T01:00:00Z");
    toSession.mockResolvedValue({ ...base, expires_at: exp });

    const state = await fetchSession("ory_session=live");
    expect(state.expiresAt).toBe(exp.getTime());
  });

  it("re-consults Kratos once a memoized session passes its expires_at", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    // Expires 30s out — inside the 60s dedup TTL, so only the expiry check can
    // evict it.
    toSession.mockResolvedValue({
      ...base,
      expires_at: new Date("2026-01-01T00:00:30Z"),
    });

    await fetchSession();
    vi.advanceTimersByTime(31_000);
    await fetchSession();

    expect(toSession).toHaveBeenCalledTimes(2);
  });

  it("still serves a session whose expiry is safely in the future", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    toSession.mockResolvedValue({
      ...base,
      expires_at: new Date("2026-01-01T01:00:00Z"),
    });

    await fetchSession();
    vi.advanceTimersByTime(30_000);
    await fetchSession();

    expect(toSession).toHaveBeenCalledTimes(1);
  });
});

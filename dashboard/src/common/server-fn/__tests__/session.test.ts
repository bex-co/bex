import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchSession } from "../session";

const toSession = vi.fn();
vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => ({ toSession }),
}));

/** How the SDK surfaces a refused whoami: the raw Response, body and all. */
const refused = (status: number, body: unknown) => ({
  response: new Response(JSON.stringify(body), { status }),
});

describe("fetchSession", () => {
  beforeEach(() => vi.clearAllMocks());

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

import { describe, it, expect } from "vitest";
import { requireAuth, EMPTY_LOGIN_SEARCH } from "../auth";

/**
 * `requireAuth` signals a redirect by throwing one (TanStack Router carries it on
 * `.options`) — run the guard and hand back the redirect it asked for, or null
 * when it let the navigation through.
 */
function guard(
  context: { session: unknown; aal2Required?: boolean },
  path?: string,
) {
  try {
    requireAuth(path)({ context });
    return null; // let through
  } catch (thrown) {
    return (thrown as { options: { to: string; search: Record<string, unknown> } })
      .options;
  }
}

describe("requireAuth", () => {
  it("lets a live session through", () => {
    expect(guard({ session: { id: "s1" } }, "/usage")).toBeNull();
  });

  it("sends an unauthenticated visitor to sign in, remembering where they were headed", () => {
    const redirect = guard({ session: null }, "/usage");

    expect(redirect?.to).toBe("/auth/login");
    expect(redirect?.search).toEqual({
      ...EMPTY_LOGIN_SEARCH,
      next: "/usage",
      aal: undefined,
    });
  });

  it("asks for the second-factor step-up when the session fetch says one is owed", () => {
    // A user who signed in with a password but owes a second factor holds a live
    // aal1 session that whoami 403s (`session_aal2_required`, docs/ADR012-auth.md
    // § MFA). They are signed in — sending them to a sign-in form would strand
    // them on `session_already_available`, so the redirect names the step-up
    // outright rather than leaving the login page to probe for it (w4/m17).
    const redirect = guard({ session: null, aal2Required: true }, "/usage");

    expect(redirect?.to).toBe("/auth/login");
    expect(redirect?.search).toMatchObject({ next: "/usage", aal: "aal2" });
  });
});

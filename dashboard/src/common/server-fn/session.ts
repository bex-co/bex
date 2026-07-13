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
};

/**
 * The Kratos session behind the current request. Pass `cookie` explicitly from a
 * server route handler, which holds the `Request` itself (`hydra-consent.ts`);
 * omit it and the ambient request's Cookie header is used.
 */
export async function fetchSession(cookie?: string): Promise<SessionState> {
  try {
    const session = await createFrontendApi(
      cookie ?? (await getRequestCookie()),
    ).toSession();
    return { session, aal2Required: false };
  } catch (err) {
    const { id } = await oryErrorInfo(err);
    return { session: null, aal2Required: id === "session_aal2_required" };
  }
}

import type {
  AcceptOAuth2LoginRequest,
  OAuth2LoginRequest,
  Session,
} from "@ory/client-fetch";
import { hydraAdmin } from "@/common/server-fn/hydra-admin";
import { fetchSession, sessionTimeMs } from "@/common/server-fn/session";

// A signed-in browser answers its own OAuth2 login challenge (w7/079,
// docs/ADR012-auth.md §7). Hydra sends every authorization to the dashboard
// login page (`urls.login`). Kratos's native `oauth2_provider` bridge accepts
// the challenge itself when Hydra's login request is skippable, which it is
// only for a browser Hydra already remembers. Every other request it turns into
// a re-authentication form (Kratos `createBrowserLoginFlow` sets `refresh` when
// `skip` is false), so a person signed in at the dashboard was asked for their
// password the first time that browser connected an agent or approved
// `bex login`. Render asks a signed-in person for nothing there.
//
// So when the browser holds a live Kratos session at the AAL Kratos requires
// (whoami answers 200 only then, `session.whoami.required_aal`), this accepts
// the challenge against that session, with the body Kratos itself sends for a
// remembered browser (hydra/hydra.go `AcceptLoginRequest`): the identity as
// subject, the session id, `amr` from the session's methods, and `remember`,
// here only until the Kratos session expires. Everything else stays with
// Kratos: a signed-out browser, a skippable request, a client that asked for a
// fresh login (`prompt=login`) or a recent one (`max_age`), a hint naming
// another person, and any request whose parameters this cannot read.
//
// Nothing is granted here. The consent route still decides every grant (a
// third-party agent's consent page, the device page's Authorize click before
// it), exactly as for a browser Hydra remembers. A challenge accepted for a
// browser that did not start it goes nowhere: Hydra's login verifier checks the
// login CSRF cookie it set on the browser that did.
//
// SERVER-ONLY: reached from the /auth/login route's GET handler through a
// dynamic import, like hydra-consent.ts.

/** Hydra's RFC 8628 verification path: a device flow's login request names it
 * as its request URL and carries no authorize parameters. */
const DEVICE_VERIFY_PATH = "/oauth2/device/verify";

/**
 * Whether the authorize request leaves the login to the session: it did not
 * ask for a fresh login or for one more recent than the session's. Hydra checks
 * neither for a browser it does not remember, so this must. Unreadable
 * parameters are a no: a POST authorize or a request object carries them
 * where the request URL does not show them.
 */
function authorizeAllowsSession(
  requestURL: string,
  authenticatedAt: number | undefined,
  now: number,
): boolean {
  let url: URL;
  try {
    url = new URL(requestURL);
  } catch {
    return false;
  }
  if (url.pathname === DEVICE_VERIFY_PATH) return true;

  const params = url.searchParams;
  if (!params.get("client_id")) return false;
  if (params.has("request") || params.has("request_uri")) return false;

  const prompts = params.getAll("prompt").flatMap((p) => p.split(" "));
  if (prompts.includes("login") || prompts.includes("none")) return false;

  for (const raw of params.getAll("max_age")) {
    if (!/^\d+$/.test(raw) || authenticatedAt === undefined) return false;
    if (authenticatedAt + Number(raw) * 1000 < now) return false;
  }
  return true;
}

/**
 * The accept body for a login request this session can answer, or null when
 * Kratos must handle it. Pure, so the rules are tested on their own.
 */
export function sessionLoginAccept(
  login: OAuth2LoginRequest,
  session: Session,
  now: number,
): AcceptOAuth2LoginRequest | null {
  // A browser Hydra remembers: Kratos accepts it with no UI already.
  if (login.skip) return null;

  const identity = session.identity?.id;
  if (!identity || !session.id || session.active === false) return null;
  // Hydra already bound this login to someone: only they may answer it.
  if (login.subject && login.subject !== identity) return null;
  const hinted = login.oidc_context?.id_token_hint_claims?.sub;
  if (hinted !== undefined && hinted !== identity) return null;

  const expiresAt = sessionTimeMs(session.expires_at);
  if (expiresAt === undefined) return null;
  // Hydra reads `remember_for: 0` as forever, so a session with under a second
  // left is not remembered at all, it is left to Kratos.
  const rememberFor = Math.floor((expiresAt - now) / 1000);
  if (rememberFor < 1) return null;

  if (
    !authorizeAllowsSession(
      login.request_url,
      sessionTimeMs(session.authenticated_at),
      now,
    )
  ) {
    return null;
  }

  return {
    subject: identity,
    identity_provider_session_id: session.id,
    amr: (session.authentication_methods ?? []).flatMap((m) =>
      m.method ? [m.method] : [],
    ),
    remember: true,
    remember_for: rememberFor,
  };
}

/**
 * The /auth/login document request. A redirect to Hydra when this browser's
 * session answers the login challenge, else null: the login page renders as
 * before and Kratos takes the challenge. Any failure is a null, so the worst
 * case is the password form this replaces.
 */
export async function handleLoginChallenge(
  request: Request,
): Promise<Response | null> {
  const loginChallenge = new URL(request.url).searchParams.get(
    "login_challenge",
  );
  const cookie = request.headers.get("cookie");
  if (!loginChallenge || !cookie) return null;
  const hydra = hydraAdmin();
  if (!hydra) return null;

  const { session } = await fetchSession(cookie);
  if (!session) return null;

  try {
    const login = await hydra.getOAuth2LoginRequest({ loginChallenge });
    const accept = sessionLoginAccept(login, session, Date.now());
    if (!accept) return null;
    const { redirect_to } = await hydra.acceptOAuth2LoginRequest({
      loginChallenge,
      acceptOAuth2LoginRequest: accept,
    });
    return Response.redirect(redirect_to, 302);
  } catch {
    // A stale or used challenge, or Hydra unreachable: the page handles both.
    return null;
  }
}

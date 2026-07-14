import { createHash, timingSafeEqual } from "node:crypto";
import { Configuration, OAuth2Api } from "@ory/client-fetch";
import type { OAuth2ConsentRequest } from "@ory/client-fetch";
import { fetchSession } from "@/common/server-fn/session";
import type { SessionState } from "@/common/server-fn/session";
import { isSameOrigin } from "@/common/server-fn/same-origin";

// OAuth2 consent (docs/ADR012-auth.md §7, w4/m9 + w4/m16). Hydra redirects the
// browser here with a consent_challenge after login (which Kratos's native
// oauth2_provider integration accepted). Hydra never skips this redirect by
// design — it ships no consent screen of its own, so the app owns the decision:
//
//   * trusted/remembered clients (Hydra `skip`, `skip_consent: true`, or the
//     operator's OAUTH_TRUSTED_CLIENTS allowlist) auto-accept headlessly, no UI;
//   * every other client — a self-registered (DCR) agent, say — renders a
//     consent page and the signed-in human approves or denies it themselves
//     (w4/m16). Approval grants exactly the requested scopes/audience and is
//     remembered for REMEMBER_FOR_SECONDS; denial rejects the flow.
//
// Every failure path denies: no lookup, no session, a session that doesn't own
// the challenge's subject, a cross-site POST ⇒ no accept call. A missing or
// stale challenge degrades to the dashboard home — never an error page a human
// has to parse (the challenge is single-use and short-TTL; a bookmarked URL
// must not strand anyone).
//
// SERVER-ONLY: talks to Hydra's admin API via @ory/client-fetch's OAuth2Api
// (the same SDK the browser side uses for Kratos's FrontendApi). Reached
// exclusively from the /auth/consent route's server handlers (dynamic import
// inside the server block), so none of this — or the env it reads — reaches
// the client bundle; only `ConsentView` (below) crosses the wire. Env is
// deliberately NOT VITE_-prefixed: `HYDRA_ADMIN_URL` (e.g. in-cluster
// http://hydra-admin.auth.svc:4445) and `OAUTH_TRUSTED_CLIENTS`
// (comma-separated client_ids).

/** How long Hydra remembers an accepted consent, so a returning user's client
 * isn't re-challenged within the window. */
const REMEMBER_FOR_SECONDS = 3600;

/** Marks a consent URL we bounced back to after a failed decision. */
const RETRY_PARAM = "retry";

/**
 * The only consent state that reaches the browser: what a human needs to decide
 * (who is asking, for what) plus the token that binds their decision back to
 * this challenge and this session. Never the raw Hydra request.
 */
export type ConsentView = {
  challenge: string;
  clientId: string;
  clientName: string;
  clientUri?: string;
  scopes: string[];
  audiences: string[];
  /** Bound to (challenge, session) — see consentCsrfToken. */
  csrfToken: string;
  /** A previous decision failed upstream, so the page says so. A bare flag, not
   * a message: everything on this page is attacker-suggestible (the whole URL
   * comes from a third-party client), so nothing from the query string is ever
   * rendered as copy. */
  retryAfterFailure: boolean;
};

function trustedClients(): Set<string> {
  return new Set(
    (process.env.OAUTH_TRUSTED_CLIENTS ?? "")
      .split(",")
      .map((c) => c.trim())
      .filter(Boolean),
  );
}

function hydraAdmin(): OAuth2Api | null {
  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  if (!admin) return null;
  return new OAuth2Api(new Configuration({ basePath: admin }));
}

/** What Kratos says about this request's cookies: the session, or why not. */
async function sessionFor(request: Request): Promise<SessionState> {
  const cookie = request.headers.get("cookie");
  return cookie
    ? fetchSession(cookie)
    : { session: null, aal2Required: false };
}

/**
 * A CSRF token a cross-site attacker cannot forge: it hashes the challenge
 * together with the Kratos **session id**, which is only readable server-side
 * with the (HttpOnly, CORS-protected) session cookie. Stateless — no extra
 * cookie to set, and it verifies identically on any dashboard replica. It is
 * defense in depth behind the same-origin check and the subject↔session
 * binding, not the only thing standing between an attacker and a grant.
 */
function consentCsrfToken(challenge: string, sessionID: string): string {
  return createHash("sha256").update(`${challenge}:${sessionID}`).digest("hex");
}

function csrfTokenMatches(
  presented: string,
  challenge: string,
  sessionID: string,
): boolean {
  const want = Buffer.from(consentCsrfToken(challenge, sessionID));
  const got = Buffer.from(presented);
  return want.length === got.length && timingSafeEqual(want, got);
}

/**
 * Require PKCE for every authorization_code grant (w6/003). Hydra mandates PKCE
 * for public clients (token_endpoint_auth_method: none) at the token endpoint,
 * but a confidential DCR client could run auth_code without it and Hydra has no
 * global "enforce PKCE" toggle — consent is the one place bex sees the flow, so
 * require a code_challenge in the original authorize request URL.
 */
function missingPKCE(consent: OAuth2ConsentRequest): boolean {
  const requestUrl = consent.request_url ? new URL(consent.request_url) : null;
  return !requestUrl?.searchParams.get("code_challenge");
}

function isTrusted(consent: OAuth2ConsentRequest): boolean {
  return (
    consent.skip === true ||
    consent.client?.skip_consent === true ||
    trustedClients().has(consent.client?.client_id ?? "")
  );
}

/** Grant exactly what was requested, remembered for the window. The one accept
 * call in the codebase — the headless (trusted) and human-approved paths grant
 * identically, so a consented client's token is indistinguishable from a
 * blessed one's downstream (docs/ADR018-render-parity.md). */
async function acceptConsent(
  hydra: OAuth2Api,
  consentChallenge: string,
  consent: OAuth2ConsentRequest,
): Promise<string> {
  const { redirect_to } = await hydra.acceptOAuth2ConsentRequest({
    consentChallenge,
    acceptOAuth2ConsentRequest: {
      grant_scope: consent.requested_scope ?? [],
      grant_access_token_audience:
        consent.requested_access_token_audience ?? [],
      remember: true,
      remember_for: REMEMBER_FOR_SECONDS,
    },
  });
  return redirect_to;
}

/** Hydra bounces the agent back to its redirect_uri with `error=access_denied`. */
async function rejectConsent(
  hydra: OAuth2Api,
  consentChallenge: string,
): Promise<string> {
  const { redirect_to } = await hydra.rejectOAuth2ConsentRequest({
    consentChallenge,
    rejectOAuth2Request: {
      error: "access_denied",
      error_description: "The user denied the request",
    },
  });
  return redirect_to;
}

/**
 * Where a consent redirect with no usable session goes: log in, then come back.
 * A session owing a second factor is not a sign-in — it is a step-up, and the
 * login page is told so outright (`aal=aal2`, w4/m17), exactly as `requireAuth`
 * does for the routed pages. Without that, the login page would refuse to mint a
 * first-factor flow (`session_already_available`), send the user back here, and
 * this would bounce them straight to login again.
 */
function loginFirst(url: URL, challenge: string, aal2Required: boolean): Response {
  const back = `/auth/consent?consent_challenge=${encodeURIComponent(challenge)}`;
  const login = new URL("/auth/login", url.origin);
  login.searchParams.set("next", back);
  if (aal2Required) login.searchParams.set("aal", "aal2");
  return Response.redirect(login.toString(), 302);
}

/**
 * Hydra's consent redirect (GET). Answers headlessly whenever it can — an
 * auto-accept, a login-first bounce, a refusal — and otherwise returns the view
 * the consent page renders, which is the caller's cue to defer to the page.
 */
export async function handleConsent(
  request: Request,
): Promise<Response | ConsentView> {
  const url = new URL(request.url);
  const home = () =>
    Response.redirect(new URL("/", url.origin).toString(), 302);

  const consentChallenge = url.searchParams.get("consent_challenge");
  if (!consentChallenge) return home();

  const hydra = hydraAdmin();
  if (!hydra) {
    return new Response("consent provider not configured", { status: 503 });
  }

  let consent: OAuth2ConsentRequest;
  try {
    consent = await hydra.getOAuth2ConsentRequest({ consentChallenge });
  } catch {
    // Stale/unknown challenge — degrade to home; a fresh authorize will mint a new one.
    return home();
  }

  if (missingPKCE(consent)) {
    return new Response(
      "PKCE required: the authorization request must include a code_challenge (S256)",
      { status: 400 },
    );
  }

  if (isTrusted(consent)) {
    try {
      return Response.redirect(
        await acceptConsent(hydra, consentChallenge, consent),
        302,
      );
    } catch {
      return new Response("consent accept failed", { status: 502 });
    }
  }

  // A third party is asking — only the human whose login minted this challenge
  // may answer it.
  const { session, aal2Required } = await sessionFor(request);
  if (!session) return loginFirst(url, consentChallenge, aal2Required);
  if (consent.subject && consent.subject !== session.identity?.id) {
    return new Response(
      "consent refused: this browser's session belongs to a different user than the one that signed in for this authorization — sign out and start over",
      { status: 403 },
    );
  }

  const clientID = consent.client?.client_id ?? "";
  return {
    challenge: consentChallenge,
    clientId: clientID,
    clientName: consent.client?.client_name || clientID,
    clientUri: consent.client?.client_uri || undefined,
    scopes: consent.requested_scope ?? [],
    audiences: consent.requested_access_token_audience ?? [],
    csrfToken: consentCsrfToken(consentChallenge, session.id),
    retryAfterFailure: url.searchParams.has(RETRY_PARAM),
  };
}

/**
 * The human's decision (POST from the consent page's form). Deny-by-default:
 * cross-site, session-less, mis-bound, or stale requests never reach an accept
 * call. Only Hydra-returned `redirect_to` values are ever redirected to — no
 * caller-supplied URL is honored anywhere in this file.
 */
export async function handleConsentDecision(
  request: Request,
): Promise<Response> {
  const url = new URL(request.url);
  const refuse = (why: string) => new Response(why, { status: 403 });

  if (!isSameOrigin(request, url)) {
    return refuse("consent refused: cross-site decision");
  }

  const form = await request.formData();
  const consentChallenge = String(form.get("consent_challenge") ?? "");
  const decision = String(form.get("decision") ?? "");
  const csrfToken = String(form.get("csrf_token") ?? "");
  if (
    !consentChallenge ||
    (decision !== "approve" && decision !== "deny") ||
    !csrfToken
  ) {
    return new Response("malformed consent decision", { status: 400 });
  }

  const hydra = hydraAdmin();
  if (!hydra) {
    return new Response("consent provider not configured", { status: 503 });
  }

  const { session } = await sessionFor(request);
  if (!session) return refuse("consent refused: no session");
  if (!csrfTokenMatches(csrfToken, consentChallenge, session.id)) {
    return refuse("consent refused: bad csrf token");
  }

  let consent: OAuth2ConsentRequest;
  try {
    consent = await hydra.getOAuth2ConsentRequest({ consentChallenge });
  } catch {
    return refuse("consent refused: stale or unknown challenge");
  }
  if (missingPKCE(consent)) {
    return new Response(
      "PKCE required: the authorization request must include a code_challenge (S256)",
      { status: 400 },
    );
  }
  if (consent.subject && consent.subject !== session.identity?.id) {
    return refuse("consent refused: session does not own this authorization");
  }

  // A failed accept/reject bounces back to the consent page with an error the
  // human can see, rather than stranding them on a bare 502 (the challenge is
  // still live — retrying is the right move).
  const retry = () => {
    const back = new URL("/auth/consent", url.origin);
    back.searchParams.set("consent_challenge", consentChallenge);
    back.searchParams.set(RETRY_PARAM, "1");
    return Response.redirect(back.toString(), 303);
  };

  try {
    const redirectTo =
      decision === "deny"
        ? await rejectConsent(hydra, consentChallenge)
        : await acceptConsent(hydra, consentChallenge, consent);
    return Response.redirect(redirectTo, 303);
  } catch {
    return retry();
  }
}

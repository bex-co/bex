import { createHash } from "node:crypto";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  handleConsent,
  handleConsentDecision,
} from "@/common/server-fn/hydra-consent";

const ADMIN = "http://hydra-admin.test:4445";
const DASHBOARD = "https://dashboard.bex.co";
const SESSION_ID = "session-abc";
const SUBJECT = "identity-xyz";
const CHALLENGE = "abc";

/** The token the consent page embeds: sha256(challenge:session id). */
const csrf = (challenge = CHALLENGE, sessionID = SESSION_ID) =>
  createHash("sha256").update(`${challenge}:${sessionID}`).digest("hex");

function consentRequest(overrides: Record<string, unknown> = {}) {
  return {
    skip: false,
    subject: SUBJECT,
    client: {
      client_id: "some-client",
      client_name: "Some Agent",
      skip_consent: false,
    },
    requested_scope: ["openid", "offline_access"],
    requested_access_token_audience: ["https://api.bex.co/mcp"],
    // request_url carries the original authorize URL incl. a PKCE code_challenge
    // (w6/003) — present by default so the existing happy-path tests pass.
    request_url:
      "https://oauth.bex.co/oauth2/auth?response_type=code&code_challenge=abc&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fevil.example%2Foauth%2Fcallback",
    ...overrides,
  };
}

function session(identityID = SUBJECT) {
  return {
    id: SESSION_ID,
    active: true,
    identity: { id: identityID, schema_id: "default", traits: {} },
  };
}

/**
 * Mock the two upstreams this module talks to: Hydra's admin API (consent
 * lookup + accept/reject, all PUTs) and Kratos's whoami (the session behind the
 * request's cookies).
 */
function mockUpstreams(opts: {
  lookupOk?: boolean;
  lookupBody?: unknown;
  acceptOk?: boolean;
  rejectOk?: boolean;
  sessionBody?: unknown;
  sessionOk?: boolean;
  /** Overrides the whoami status — e.g. 403, a live session owing a second factor. */
  sessionStatus?: number;
}) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url.includes("/sessions/whoami")) {
        return new Response(JSON.stringify(opts.sessionBody ?? session()), {
          status: opts.sessionStatus ?? (opts.sessionOk === false ? 401 : 200),
        });
      }
      if (url.includes("/consent/accept")) {
        return new Response(
          JSON.stringify({ redirect_to: "https://oauth.bex.co/continue" }),
          { status: opts.acceptOk === false ? 500 : 200 },
        );
      }
      if (url.includes("/consent/reject")) {
        return new Response(
          JSON.stringify({ redirect_to: "https://oauth.bex.co/denied" }),
          { status: opts.rejectOk === false ? 500 : 200 },
        );
      }
      return new Response(JSON.stringify(opts.lookupBody ?? {}), {
        status: opts.lookupOk === false ? 404 : 200,
      });
    }),
  );
  return calls;
}

const accepts = (calls: { url: string }[]) =>
  calls.filter((c) => c.url.includes("/consent/accept"));
const rejects = (calls: { url: string }[]) =>
  calls.filter((c) => c.url.includes("/consent/reject"));

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
  delete process.env.OAUTH_TRUSTED_CLIENTS;
});
afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.HYDRA_ADMIN_URL;
  delete process.env.OAUTH_TRUSTED_CLIENTS;
});

/** A consent GET as the browser makes it: session cookie, challenge in the query. */
const req = (qs: string, cookie: string | null = "ory_session=live") =>
  new Request(`${DASHBOARD}/auth/consent${qs}`, {
    headers: cookie ? { cookie } : {},
  });

/** The consent POST the page's form makes: same-origin, form-encoded decision. */
const decisionReq = (
  fields: Record<string, string>,
  opts: { origin?: string | null; cookie?: string | null } = {},
) => {
  const body = new URLSearchParams(fields);
  const headers: Record<string, string> = {
    "content-type": "application/x-www-form-urlencoded",
  };
  const origin = opts.origin === undefined ? DASHBOARD : opts.origin;
  if (origin) headers.origin = origin;
  const cookie = opts.cookie === undefined ? "ory_session=live" : opts.cookie;
  if (cookie) headers.cookie = cookie;
  return new Request(`${DASHBOARD}/auth/consent`, {
    method: "POST",
    headers,
    body,
  });
};

describe("handleConsent (GET)", () => {
  it("auto-accepts when Hydra says skip=true and grants the requested scopes", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest({ skip: true }) });

    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));

    expect(res).toBeInstanceOf(Response);
    expect((res as Response).status).toBe(302);
    expect((res as Response).headers.get("Location")).toBe(
      "https://oauth.bex.co/continue",
    );
    const body = JSON.parse(accepts(calls)[0].init?.body as string);
    expect(body.grant_scope).toEqual(["openid", "offline_access"]);
    expect(body.grant_access_token_audience).toEqual([
      "https://api.bex.co/mcp",
    ]);
    expect(body.remember).toBe(true);
  });

  it("auto-accepts a client Hydra marks skip_consent", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest({
        client: { client_id: "bex-dashboard", skip_consent: true },
      }),
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(302);
    expect(accepts(calls)).toHaveLength(1);
  });

  it("auto-accepts a client from the OAUTH_TRUSTED_CLIENTS allowlist", async () => {
    process.env.OAUTH_TRUSTED_CLIENTS = "agent-one, some-client";
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(302);
    expect(accepts(calls)).toHaveLength(1);
  });

  it("renders the consent page for an unknown client with a live session (w4/m16)", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });

    const view = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));

    expect(accepts(calls)).toHaveLength(0);
    expect(view).not.toBeInstanceOf(Response);
    expect(view).toMatchObject({
      challenge: CHALLENGE,
      clientId: "some-client",
      clientName: "Some Agent",
      redirectOrigin: "https://evil.example",
      scopes: ["openid", "offline_access"],
      audiences: ["https://api.bex.co/mcp"],
      csrfToken: csrf(),
      retryAfterFailure: false,
    });
  });

  it("flags a consent page bounced back after a failed decision", async () => {
    mockUpstreams({ lookupBody: consentRequest() });
    const view = await handleConsent(
      req(`?consent_challenge=${CHALLENGE}&retry=1`),
    );
    expect(view).toMatchObject({ retryAfterFailure: true });
  });

  it("falls back to the client_id when the client has no name", async () => {
    mockUpstreams({
      lookupBody: consentRequest({ client: { client_id: "nameless" } }),
    });
    const view = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect(view).toMatchObject({ clientName: "nameless" });
  });

  it("sends an unauthenticated browser to log in first, then back to consent", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsent(
      req(`?consent_challenge=${CHALLENGE}`, null),
    );
    const location = (res as Response).headers.get("Location")!;
    expect(location).toContain("/auth/login");
    expect(new URL(location).searchParams.get("next")).toBe(
      `/auth/consent?consent_challenge=${CHALLENGE}`,
    );
    expect(accepts(calls)).toHaveLength(0);
  });

  it("sends a browser owing a second factor to the step-up, not to a sign-in form", async () => {
    // Under `highest_available` (docs/ADR012-auth.md § MFA) this browser holds a live
    // aal1 session that whoami 403s. A plain login bounce would strand it: the
    // login page can't mint a first-factor flow against a live session, would send
    // it back here, and this would bounce it to login again. Name the step-up (w4/m17).
    const calls = mockUpstreams({
      lookupBody: consentRequest(),
      sessionStatus: 403,
      sessionBody: { error: { id: "session_aal2_required" } },
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    const search = new URL((res as Response).headers.get("Location")!)
      .searchParams;
    expect(search.get("aal")).toBe("aal2");
    expect(search.get("next")).toBe(
      `/auth/consent?consent_challenge=${CHALLENGE}`,
    );
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses to render when the session belongs to another user than the challenge's subject", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest(),
      sessionBody: session("someone-else"),
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("rejects an authorization_code grant without a PKCE code_challenge (w6/003)", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest({
        request_url: "https://oauth.bex.co/oauth2/auth?response_type=code",
      }),
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(400);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("allows Hydra's device flow through consent without inventing PKCE", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest({
        request_url:
          "https://oauth.bex.co/oauth2/device/verify?device_verifier=v&client_id=429024F5E608930E2A65EF92591A25CC",
      }),
    });
    const view = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect(view).not.toBeInstanceOf(Response);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("degrades to home on a missing challenge", async () => {
    mockUpstreams({});
    const res = await handleConsent(req(""));
    expect((res as Response).status).toBe(302);
    expect((res as Response).headers.get("Location")).toBe(`${DASHBOARD}/`);
  });

  it("degrades to home on a stale/unknown challenge", async () => {
    mockUpstreams({ lookupOk: false });
    const res = await handleConsent(req("?consent_challenge=stale"));
    expect((res as Response).status).toBe(302);
    expect((res as Response).headers.get("Location")).toBe(`${DASHBOARD}/`);
  });

  it("answers 503 when the admin URL is not configured", async () => {
    delete process.env.HYDRA_ADMIN_URL;
    mockUpstreams({});
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(503);
  });

  it("answers 502 when the headless accept fails upstream", async () => {
    mockUpstreams({
      lookupBody: consentRequest({ skip: true }),
      acceptOk: false,
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(502);
  });
});

describe("handleConsentDecision (POST)", () => {
  const approve = {
    consent_challenge: CHALLENGE,
    decision: "approve",
    csrf_token: csrf(),
  };

  it("approves: grants the requested scopes + audience, remembers, and returns to Hydra", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });

    const res = await handleConsentDecision(decisionReq(approve));

    expect(res.status).toBe(303);
    expect(res.headers.get("Location")).toBe("https://oauth.bex.co/continue");
    const body = JSON.parse(accepts(calls)[0].init?.body as string);
    expect(body.grant_scope).toEqual(["openid", "offline_access"]);
    expect(body.grant_access_token_audience).toEqual([
      "https://api.bex.co/mcp",
    ]);
    expect(body.remember).toBe(true);
    expect(body.remember_for).toBeGreaterThan(0);
  });

  it("denies: rejects the request with access_denied and never accepts", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });

    const res = await handleConsentDecision(
      decisionReq({ ...approve, decision: "deny" }),
    );

    expect(res.status).toBe(303);
    expect(res.headers.get("Location")).toBe("https://oauth.bex.co/denied");
    expect(accepts(calls)).toHaveLength(0);
    const body = JSON.parse(rejects(calls)[0].init?.body as string);
    expect(body.error).toBe("access_denied");
  });

  it("refuses a cross-site POST (foreign Origin) without touching Hydra", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq(approve, { origin: "https://evil.example" }),
    );
    expect(res.status).toBe(403);
    expect(calls).toHaveLength(0);
  });

  it("refuses a POST with no Origin header at all", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq(approve, { origin: null }),
    );
    expect(res.status).toBe(403);
    expect(calls).toHaveLength(0);
  });

  it("accepts the browser's https Origin behind an http ingress (x-forwarded-host)", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const body = new URLSearchParams(approve);
    const res = await handleConsentDecision(
      new Request("http://10.0.0.5:3000/auth/consent", {
        method: "POST",
        headers: {
          "content-type": "application/x-www-form-urlencoded",
          origin: DASHBOARD,
          "x-forwarded-host": "dashboard.bex.co",
          cookie: "ory_session=live",
        },
        body,
      }),
    );
    expect(res.status).toBe(303);
    expect(accepts(calls)).toHaveLength(1);
  });

  it("refuses a forged CSRF token", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq({ ...approve, csrf_token: "not-the-token" }),
    );
    expect(res.status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses a CSRF token minted for a different challenge", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq({ ...approve, csrf_token: csrf("other-challenge") }),
    );
    expect(res.status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses when the session's identity is not the challenge's subject", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest(),
      sessionBody: session("someone-else"),
    });
    const res = await handleConsentDecision(decisionReq(approve));
    expect(res.status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses when there is no session", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq(approve, { cookie: null }),
    );
    expect(res.status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses a stale/unknown challenge", async () => {
    const calls = mockUpstreams({ lookupOk: false });
    const res = await handleConsentDecision(decisionReq(approve));
    expect(res.status).toBe(403);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("refuses an approve for a flow that never carried PKCE", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest({
        request_url: "https://oauth.bex.co/oauth2/auth?response_type=code",
      }),
    });
    const res = await handleConsentDecision(decisionReq(approve));
    expect(res.status).toBe(400);
    expect(accepts(calls)).toHaveLength(0);
  });

  it("400s a malformed decision", async () => {
    const calls = mockUpstreams({ lookupBody: consentRequest() });
    const res = await handleConsentDecision(
      decisionReq({ ...approve, decision: "maybe" }),
    );
    expect(res.status).toBe(400);
    // A malformed decision reaches no consent accept/reject/lookup — only the
    // session whoami (now fetched before the body is buffered, codex-security #11).
    expect(calls.filter((c) => c.url.includes("/consent/"))).toHaveLength(0);
  });

  it("bounces back to the consent page with a visible error when Hydra's accept fails", async () => {
    mockUpstreams({ lookupBody: consentRequest(), acceptOk: false });
    const res = await handleConsentDecision(decisionReq(approve));
    expect(res.status).toBe(303);
    const location = new URL(res.headers.get("Location")!);
    expect(location.pathname).toBe("/auth/consent");
    expect(location.searchParams.has("retry")).toBe(true);
    expect(location.searchParams.get("consent_challenge")).toBe(CHALLENGE);
  });
});

// w1/m66 F8: the gate documented itself as enforcing S256 but only checked that
// a code_challenge parameter existed, so `plain` (and an omitted method, which
// RFC 7636 defines AS plain) reached consent acceptance. Both entry points must
// refuse anything that is not exactly one S256 challenge — a downgrade here
// removes the protection an intercepted authorization request relies on.
describe("PKCE S256 enforcement (w1/m66 F8)", () => {
  const authorize = "https://oauth.bex.co/oauth2/auth?response_type=code";
  const approve = {
    consent_challenge: CHALLENGE,
    decision: "approve",
    csrf_token: csrf(),
  };

  const refused: Array<[string, string]> = [
    ["plain", `${authorize}&code_challenge=abc&code_challenge_method=plain`],
    ["omitted method (RFC 7636 => plain)", `${authorize}&code_challenge=abc`],
    [
      "lowercase s256",
      `${authorize}&code_challenge=abc&code_challenge_method=s256`,
    ],
    // %20, not a literal space: the WHATWG URL parser strips trailing spaces
    // from the input string, so a literal one would never reach the check.
    [
      "trailing whitespace in the method",
      `${authorize}&code_challenge=abc&code_challenge_method=S256%20`,
    ],
    [
      "duplicated method",
      `${authorize}&code_challenge=abc&code_challenge_method=S256&code_challenge_method=plain`,
    ],
    [
      "duplicated challenge",
      `${authorize}&code_challenge=abc&code_challenge=def&code_challenge_method=S256`,
    ],
    [
      "empty challenge",
      `${authorize}&code_challenge=&code_challenge_method=S256`,
    ],
    [
      "unknown method",
      `${authorize}&code_challenge=abc&code_challenge_method=SHA256`,
    ],
    ["unparseable authorize URL", "not-a-url"],
  ];

  for (const [label, request_url] of refused) {
    it(`refuses ${label} on the headless/trusted path`, async () => {
      const calls = mockUpstreams({
        // skip: true would auto-accept — prove the PKCE gate runs first.
        lookupBody: consentRequest({ request_url, skip: true }),
      });
      const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
      expect((res as Response).status).toBe(400);
      expect(accepts(calls)).toHaveLength(0);
    });

    it(`refuses ${label} on the human-decision path`, async () => {
      const calls = mockUpstreams({
        lookupBody: consentRequest({ request_url }),
      });
      const res = await handleConsentDecision(decisionReq(approve));
      expect(res.status).toBe(400);
      expect(accepts(calls)).toHaveLength(0);
    });
  }

  it("still accepts an exact S256 request on both paths", async () => {
    const request_url = `${authorize}&code_challenge=abc&code_challenge_method=S256`;
    const headless = mockUpstreams({
      lookupBody: consentRequest({ request_url, skip: true }),
    });
    const res = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect((res as Response).status).toBe(302);
    expect(accepts(headless)).toHaveLength(1);

    const human = mockUpstreams({
      lookupBody: consentRequest({ request_url }),
    });
    const decided = await handleConsentDecision(decisionReq(approve));
    expect(decided.status).toBe(303);
    expect(accepts(human)).toHaveLength(1);
  });

  it("keeps the RFC 8628 device exception (the official Render CLI login)", async () => {
    const calls = mockUpstreams({
      lookupBody: consentRequest({
        request_url:
          "https://oauth.bex.co/oauth2/device/verify?device_verifier=v&client_id=429024F5E608930E2A65EF92591A25CC",
      }),
    });
    const view = await handleConsent(req(`?consent_challenge=${CHALLENGE}`));
    expect(view).not.toBeInstanceOf(Response);
    expect(accepts(calls)).toHaveLength(0);
  });
});

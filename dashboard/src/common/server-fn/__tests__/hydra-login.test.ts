// @vitest-environment node
// Node, as in production: the handler runs only on the dashboard's server.
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { OAuth2LoginRequest, Session } from "@ory/client-fetch";
import {
  handleLoginChallenge,
  sessionLoginAccept,
} from "@/common/server-fn/hydra-login";

const ADMIN = "http://hydra-admin.test:4445";
const DASHBOARD = "https://dashboard.bex.co";
const ISSUER = "https://oauth.bex.co";
const CHALLENGE = "login-challenge-1";
const SESSION_ID = "session-abc";
const IDENTITY = "identity-xyz";
const NOW = Date.parse("2026-11-17T12:00:00Z");
/** Production's lifespan is 168h; six days of it are left. */
const EXPIRES_AT = "2026-11-23T12:00:00Z";
const SIX_DAYS = 6 * 24 * 3600;
const AUTHORIZE = `${ISSUER}/oauth2/auth?client_id=agent&response_type=code&scope=openid&redirect_uri=http%3A%2F%2F127.0.0.1%3A9876%2Fcb&state=s&code_challenge=c&code_challenge_method=S256`;

/** A login request as Hydra hands it over for a browser it does not remember. */
function loginRequest(
  overrides: Partial<OAuth2LoginRequest> = {},
): OAuth2LoginRequest {
  return {
    challenge: CHALLENGE,
    client: { client_id: "agent" },
    request_url: AUTHORIZE,
    skip: false,
    subject: "",
    ...overrides,
  };
}

/** A live Kratos session as whoami answers it, signed in an hour ago. */
function session(overrides: Record<string, unknown> = {}) {
  return {
    id: SESSION_ID,
    active: true,
    authenticated_at: "2026-11-17T11:00:00Z",
    expires_at: EXPIRES_AT,
    identity: { id: IDENTITY, schema_id: "default", traits: {} },
    authentication_methods: [{ method: "password", aal: "aal1" }],
    ...overrides,
  };
}

/** The SDK's typed form of a session, as fetchSession returns it. */
function typedSession(overrides: Record<string, unknown> = {}): Session {
  const raw = session(overrides);
  return {
    ...raw,
    authenticated_at: new Date(raw.authenticated_at as string),
    expires_at: new Date(raw.expires_at as string),
  } as Session;
}

const authorizeWith = (params: string) => `${AUTHORIZE}&${params}`;

describe("sessionLoginAccept", () => {
  it("answers a login Hydra cannot skip with the body Kratos sends for a remembered browser", () => {
    expect(sessionLoginAccept(loginRequest(), typedSession(), NOW)).toEqual({
      subject: IDENTITY,
      identity_provider_session_id: SESSION_ID,
      amr: ["password"],
      remember: true,
      remember_for: SIX_DAYS,
    });
  });

  it("leaves a browser Hydra remembers to Kratos, which accepts it already", () => {
    expect(
      sessionLoginAccept(
        loginRequest({ skip: true, subject: IDENTITY }),
        typedSession(),
        NOW,
      ),
    ).toBeNull();
  });

  it("answers the RFC 8628 device flow, whose request URL is Hydra's verifier", () => {
    const accept = sessionLoginAccept(
      loginRequest({ request_url: `${ISSUER}/oauth2/device/verify` }),
      typedSession(),
      NOW,
    );
    expect(accept?.subject).toBe(IDENTITY);
  });

  it("names every method the session used in amr, a second factor included", () => {
    const accept = sessionLoginAccept(
      loginRequest(),
      typedSession({
        authentication_methods: [
          { method: "password", aal: "aal1" },
          { method: "totp", aal: "aal2" },
        ],
      }),
      NOW,
    );
    expect(accept?.amr).toEqual(["password", "totp"]);
  });

  it("reads timestamps given as strings too", () => {
    const accept = sessionLoginAccept(
      loginRequest(),
      session() as unknown as Session,
      NOW,
    );
    expect(accept?.remember_for).toBe(SIX_DAYS);
  });

  describe("a client that asked for a fresh login keeps Kratos's form", () => {
    it.each([
      ["prompt=login"],
      ["prompt=consent%20login"],
      ["prompt=consent&prompt=login"],
      ["prompt=none"],
    ])("%s", (params) => {
      expect(
        sessionLoginAccept(
          loginRequest({ request_url: authorizeWith(params) }),
          typedSession(),
          NOW,
        ),
      ).toBeNull();
    });

    it("but prompt=consent alone leaves the login to the session", () => {
      expect(
        sessionLoginAccept(
          loginRequest({ request_url: authorizeWith("prompt=consent") }),
          typedSession(),
          NOW,
        ),
      ).not.toBeNull();
    });
  });

  describe("max_age is checked against the Kratos session's sign-in", () => {
    it("answers when the session signed in within it", () => {
      expect(
        sessionLoginAccept(
          loginRequest({ request_url: authorizeWith("max_age=3600") }),
          typedSession(),
          NOW,
        ),
      ).not.toBeNull();
    });

    it.each([
      ["older than max_age", "max_age=60", {}],
      ["max_age=0", "max_age=0", {}],
      ["a malformed max_age", "max_age=soon", {}],
      ["a negative max_age", "max_age=-1", {}],
      [
        "no sign-in time to judge it by",
        "max_age=3600",
        { authenticated_at: undefined },
      ],
    ])("leaves %s to Kratos", (_what, params, sessionOverrides) => {
      expect(
        sessionLoginAccept(
          loginRequest({ request_url: authorizeWith(params) }),
          { ...typedSession(), ...sessionOverrides } as Session,
          NOW,
        ),
      ).toBeNull();
    });
  });

  describe("a request bound to someone else is not this session's to answer", () => {
    it("leaves a subject Hydra already bound to another identity", () => {
      expect(
        sessionLoginAccept(
          loginRequest({ subject: "someone-else" }),
          typedSession(),
          NOW,
        ),
      ).toBeNull();
    });

    it("answers a subject that is this identity", () => {
      expect(
        sessionLoginAccept(
          loginRequest({ subject: IDENTITY }),
          typedSession(),
          NOW,
        ),
      ).not.toBeNull();
    });

    it("leaves an id_token_hint naming another person", () => {
      expect(
        sessionLoginAccept(
          loginRequest({
            oidc_context: { id_token_hint_claims: { sub: "someone-else" } },
          }),
          typedSession(),
          NOW,
        ),
      ).toBeNull();
    });
  });

  describe("parameters it cannot read leave the login to Kratos", () => {
    it.each([
      ["an unparseable request URL", "not a url"],
      [
        "a POST authorize, whose URL carries no client_id",
        `${ISSUER}/oauth2/auth`,
      ],
      ["a request object", authorizeWith("request=eyJhbGciOi")],
      ["a request_uri", authorizeWith("request_uri=urn%3Aexample")],
    ])("%s", (_what, requestURL) => {
      expect(
        sessionLoginAccept(
          loginRequest({ request_url: requestURL }),
          typedSession(),
          NOW,
        ),
      ).toBeNull();
    });
  });

  describe("a session that cannot be remembered is not used", () => {
    it.each([
      ["no expiry", { expires_at: undefined }],
      ["already expired", { expires_at: new Date(NOW - 1000) }],
      [
        "under a second left (remember_for 0 is forever)",
        { expires_at: new Date(NOW + 500) },
      ],
      ["inactive", { active: false }],
      ["no identity", { identity: undefined }],
      ["no session id", { id: "" }],
    ])("%s", (_what, overrides) => {
      expect(
        sessionLoginAccept(
          loginRequest(),
          { ...typedSession(), ...overrides } as Session,
          NOW,
        ),
      ).toBeNull();
    });
  });
});

/**
 * Mock the two upstreams: Kratos's whoami (the browser's session) and Hydra's
 * admin API (the login request and its accept).
 */
function mockUpstreams(
  opts: {
    sessionStatus?: number;
    sessionBody?: unknown;
    lookupStatus?: number;
    lookupBody?: unknown;
    acceptStatus?: number;
  } = {},
) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url.includes("/sessions/whoami")) {
        const status = opts.sessionStatus ?? 200;
        return new Response(
          JSON.stringify(
            status === 200
              ? (opts.sessionBody ?? session())
              : { error: { id: "session_inactive" } },
          ),
          { status },
        );
      }
      if (url.includes("/requests/login/accept")) {
        return new Response(
          JSON.stringify({
            redirect_to: `${ISSUER}/oauth2/auth?login_verifier=v`,
          }),
          { status: opts.acceptStatus ?? 200 },
        );
      }
      if (url.includes("/requests/login")) {
        return new Response(JSON.stringify(opts.lookupBody ?? loginRequest()), {
          status: opts.lookupStatus ?? 200,
        });
      }
      return new Response("unexpected", { status: 599 });
    }),
  );
  return calls;
}

const hydraCalls = (calls: { url: string }[]) =>
  calls.filter((c) => c.url.startsWith(ADMIN));
const acceptCalls = (calls: { url: string; init?: RequestInit }[]) =>
  calls.filter((c) => c.url.includes("/requests/login/accept"));

/** The login page's document request as Hydra's redirect makes it. */
const req = (
  qs = `?login_challenge=${CHALLENGE}`,
  cookie: string | null = "ory_kratos_session=live",
) =>
  new Request(`${DASHBOARD}/auth/login${qs}`, {
    headers: cookie ? { cookie } : {},
  });

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
  vi.useFakeTimers({ now: NOW, toFake: ["Date"] });
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  delete process.env.HYDRA_ADMIN_URL;
});

describe("handleLoginChallenge", () => {
  it("accepts the challenge for a signed-in browser and sends it on to Hydra", async () => {
    const calls = mockUpstreams();

    const res = await handleLoginChallenge(req());

    expect(res?.status).toBe(302);
    expect(res?.headers.get("Location")).toBe(
      `${ISSUER}/oauth2/auth?login_verifier=v`,
    );
    const [accept] = acceptCalls(calls);
    expect(accept.init?.method).toBe("PUT");
    expect(new URL(accept.url).searchParams.get("login_challenge")).toBe(
      CHALLENGE,
    );
    expect(JSON.parse(accept.init?.body as string)).toEqual({
      subject: IDENTITY,
      identity_provider_session_id: SESSION_ID,
      amr: ["password"],
      remember: true,
      remember_for: SIX_DAYS,
    });
    // The browser's own cookie is what proves the session.
    const whoami = calls.find((c) => c.url.includes("/sessions/whoami"));
    expect(new Headers(whoami?.init?.headers).get("cookie")).toBe(
      "ory_kratos_session=live",
    );
  });

  it("renders the page for a visit with no login challenge, asking nobody", async () => {
    const calls = mockUpstreams();
    expect(await handleLoginChallenge(req(""))).toBeNull();
    expect(calls).toHaveLength(0);
  });

  it("renders the page for a browser with no cookies, asking nobody", async () => {
    const calls = mockUpstreams();
    expect(await handleLoginChallenge(req(undefined, null))).toBeNull();
    expect(calls).toHaveLength(0);
  });

  it("renders the page when Hydra's admin URL is not configured", async () => {
    delete process.env.HYDRA_ADMIN_URL;
    const calls = mockUpstreams();
    expect(await handleLoginChallenge(req())).toBeNull();
    expect(calls).toHaveLength(0);
  });

  it.each([
    ["signed out", 401],
    ["owing a second factor (whoami 403 session_aal2_required)", 403],
  ])(
    "leaves a browser %s to Kratos, without asking Hydra",
    async (_what, status) => {
      const calls = mockUpstreams({ sessionStatus: status });
      expect(await handleLoginChallenge(req())).toBeNull();
      expect(hydraCalls(calls)).toHaveLength(0);
    },
  );

  it("leaves a request Kratos must answer, accepting nothing", async () => {
    const calls = mockUpstreams({
      lookupBody: loginRequest({ request_url: authorizeWith("prompt=login") }),
    });
    expect(await handleLoginChallenge(req())).toBeNull();
    expect(acceptCalls(calls)).toHaveLength(0);
  });

  it("leaves a stale or unknown challenge to the page", async () => {
    const calls = mockUpstreams({ lookupStatus: 404 });
    expect(await handleLoginChallenge(req())).toBeNull();
    expect(acceptCalls(calls)).toHaveLength(0);
  });

  it("falls back to the page when Hydra refuses the accept", async () => {
    mockUpstreams({ acceptStatus: 409 });
    expect(await handleLoginChallenge(req())).toBeNull();
  });
});

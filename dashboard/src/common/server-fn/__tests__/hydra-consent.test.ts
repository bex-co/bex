import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { handleConsent } from "@/common/server-fn/hydra-consent";

const ADMIN = "http://hydra-admin.test:4445";

function consentRequest(overrides: Record<string, unknown> = {}) {
  return {
    skip: false,
    client: { client_id: "some-client", skip_consent: false },
    requested_scope: ["openid", "offline_access"],
    requested_access_token_audience: ["https://api.bex.co/mcp"],
    // request_url carries the original authorize URL incl. a PKCE code_challenge
    // (w6/003) — present by default so the existing happy-path tests pass.
    request_url:
      "https://oauth.bex.co/oauth2/auth?response_type=code&code_challenge=abc&code_challenge_method=S256",
    ...overrides,
  };
}

/** Mock fetch: GET consent lookup → lookupBody (or !ok), PUT accept → acceptBody. */
function mockHydra(opts: {
  lookupOk?: boolean;
  lookupBody?: unknown;
  acceptOk?: boolean;
  acceptBody?: unknown;
}) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (init?.method === "PUT") {
        return new Response(JSON.stringify(opts.acceptBody ?? {}), {
          status: opts.acceptOk === false ? 500 : 200,
        });
      }
      return new Response(JSON.stringify(opts.lookupBody ?? {}), {
        status: opts.lookupOk === false ? 404 : 200,
      });
    }),
  );
  return calls;
}

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
  delete process.env.OAUTH_TRUSTED_CLIENTS;
});
afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.HYDRA_ADMIN_URL;
  delete process.env.OAUTH_TRUSTED_CLIENTS;
});

const req = (qs: string) =>
  new Request(`https://dashboard.bex.co/auth/consent${qs}`);

describe("handleConsent", () => {
  it("auto-accepts when Hydra says skip=true and grants the requested scopes", async () => {
    const calls = mockHydra({
      lookupBody: consentRequest({ skip: true }),
      acceptBody: { redirect_to: "https://oauth.bex.co/continue" },
    });

    const res = await handleConsent(req("?consent_challenge=abc"));

    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe("https://oauth.bex.co/continue");
    const accept = calls.find((c) => c.init?.method === "PUT");
    expect(accept?.url).toContain("/admin/oauth2/auth/requests/consent/accept");
    const body = JSON.parse(accept?.init?.body as string);
    expect(body.grant_scope).toEqual(["openid", "offline_access"]);
    expect(body.grant_access_token_audience).toEqual([
      "https://api.bex.co/mcp",
    ]);
  });

  it("auto-accepts a client Hydra marks skip_consent", async () => {
    mockHydra({
      lookupBody: consentRequest({
        client: { client_id: "bex-dashboard", skip_consent: true },
      }),
      acceptBody: { redirect_to: "https://oauth.bex.co/continue" },
    });
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(302);
  });

  it("auto-accepts a client from the OAUTH_TRUSTED_CLIENTS allowlist", async () => {
    process.env.OAUTH_TRUSTED_CLIENTS = "agent-one, some-client";
    mockHydra({
      lookupBody: consentRequest(),
      acceptBody: { redirect_to: "https://oauth.bex.co/continue" },
    });
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(302);
  });

  it("denies an unknown client with 403 and never calls accept", async () => {
    const calls = mockHydra({ lookupBody: consentRequest() });
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(403);
    expect(calls.some((c) => c.init?.method === "PUT")).toBe(false);
  });

  it("rejects an authorization_code grant without a PKCE code_challenge (w6/003)", async () => {
    const calls = mockHydra({
      lookupBody: consentRequest({
        request_url: "https://oauth.bex.co/oauth2/auth?response_type=code",
      }),
    });
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(400);
    expect(calls.some((c) => c.init?.method === "PUT")).toBe(false);
  });

  it("degrades to home on a missing challenge", async () => {
    mockHydra({});
    const res = await handleConsent(req(""));
    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe("https://dashboard.bex.co/");
  });

  it("degrades to home on a stale/unknown challenge", async () => {
    mockHydra({ lookupOk: false });
    const res = await handleConsent(req("?consent_challenge=stale"));
    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe("https://dashboard.bex.co/");
  });

  it("answers 503 when the admin URL is not configured", async () => {
    delete process.env.HYDRA_ADMIN_URL;
    mockHydra({});
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(503);
  });

  it("answers 502 when the accept call fails upstream", async () => {
    mockHydra({ lookupBody: consentRequest({ skip: true }), acceptOk: false });
    const res = await handleConsent(req("?consent_challenge=abc"));
    expect(res.status).toBe(502);
  });
});

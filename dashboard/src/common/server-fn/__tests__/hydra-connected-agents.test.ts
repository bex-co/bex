import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  listConnectedAgents,
  revokeConnectedAgent,
} from "@/common/server-fn/hydra-connected-agents";

const ADMIN = "http://hydra-admin.test:4445";
const DASHBOARD = "https://dashboard.bex.co";
const SUBJECT = "identity-xyz";

function consentSession(overrides: Record<string, unknown> = {}) {
  return {
    consent_request: {
      client: {
        client_id: "some-client",
        client_name: "Some Agent",
        client_uri: "https://agent.example",
      },
    },
    grant_scope: ["openid", "offline_access"],
    handled_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function session(identityID = SUBJECT) {
  return {
    id: "session-abc",
    active: true,
    identity: { id: identityID, schema_id: "default", traits: {} },
  };
}

/** Mock the two upstreams this module talks to: Kratos whoami + Hydra's admin consent-session API. */
function mockUpstreams(opts: {
  sessionBody?: unknown;
  sessionOk?: boolean;
  consentSessions?: unknown[];
  listOk?: boolean;
  revokeOk?: boolean;
}) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url.includes("/sessions/whoami")) {
        return new Response(JSON.stringify(opts.sessionBody ?? session()), {
          status: opts.sessionOk === false ? 401 : 200,
        });
      }
      if (url.includes("/admin/oauth2/auth/sessions/consent")) {
        if (init?.method === "DELETE") {
          return new Response(null, {
            status: opts.revokeOk === false ? 500 : 204,
          });
        }
        return new Response(
          JSON.stringify(opts.consentSessions ?? [consentSession()]),
          { status: opts.listOk === false ? 500 : 200 },
        );
      }
      return new Response("{}", { status: 404 });
    }),
  );
  return calls;
}

const req = (cookie: string | null = "ory_session=live") =>
  new Request(`${DASHBOARD}/api/connected-agents`, {
    headers: cookie ? { cookie } : {},
  });

const postReq = (
  body: Record<string, unknown>,
  opts: { origin?: string | null; cookie?: string | null } = {},
) => {
  const origin = opts.origin === undefined ? DASHBOARD : opts.origin;
  const cookie = opts.cookie === undefined ? "ory_session=live" : opts.cookie;
  const headers: Record<string, string> = { "content-type": "application/json" };
  if (origin) headers.origin = origin;
  if (cookie) headers.cookie = cookie;
  return new Request(`${DASHBOARD}/api/connected-agents`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
};

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
});
afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.HYDRA_ADMIN_URL;
});

describe("listConnectedAgents", () => {
  it("returns an empty list with no session", async () => {
    mockUpstreams({ sessionOk: false });
    const res = await listConnectedAgents(req(null));
    expect(await res.json()).toEqual([]);
  });

  it("503s when Hydra admin isn't configured", async () => {
    delete process.env.HYDRA_ADMIN_URL;
    mockUpstreams({});
    const res = await listConnectedAgents(req());
    expect(res.status).toBe(503);
  });

  it("lists one row per client, with its granted scopes and grant date", async () => {
    mockUpstreams({});
    const res = await listConnectedAgents(req());
    const body = await res.json();
    expect(body).toEqual([
      {
        clientId: "some-client",
        clientName: "Some Agent",
        clientUri: "https://agent.example",
        scopes: ["openid", "offline_access"],
        grantedAt: "2026-01-01T00:00:00.000Z",
      },
    ]);
  });

  it("merges multiple consent sessions for the same client into one row", async () => {
    mockUpstreams({
      consentSessions: [
        consentSession({
          grant_scope: ["openid"],
          handled_at: "2026-01-01T00:00:00.000Z",
        }),
        consentSession({
          grant_scope: ["offline_access"],
          handled_at: "2026-02-01T00:00:00.000Z",
        }),
      ],
    });
    const res = await listConnectedAgents(req());
    const body = await res.json();
    expect(body).toHaveLength(1);
    expect(body[0].scopes.sort()).toEqual(["offline_access", "openid"]);
    expect(body[0].grantedAt).toBe("2026-02-01T00:00:00.000Z");
  });
});

describe("revokeConnectedAgent", () => {
  it("refuses a cross-site request", async () => {
    mockUpstreams({});
    const res = await revokeConnectedAgent(
      postReq({ clientId: "some-client" }, { origin: "https://evil.example" }),
    );
    expect(res.status).toBe(403);
  });

  it("refuses with no session", async () => {
    mockUpstreams({ sessionOk: false });
    const res = await revokeConnectedAgent(
      postReq({ clientId: "some-client" }),
    );
    expect(res.status).toBe(401);
  });

  it("revokes the subject's consent sessions for the given client", async () => {
    const calls = mockUpstreams({});
    const res = await revokeConnectedAgent(
      postReq({ clientId: "some-client" }),
    );
    expect(res.status).toBe(204);
    const revoke = calls.find(
      (c) =>
        c.url.includes("/admin/oauth2/auth/sessions/consent") &&
        c.init?.method === "DELETE",
    );
    expect(revoke?.url).toContain(`subject=${SUBJECT}`);
    expect(revoke?.url).toContain("client=some-client");
  });

  it("502s when the revoke call fails upstream", async () => {
    mockUpstreams({ revokeOk: false });
    const res = await revokeConnectedAgent(
      postReq({ clientId: "some-client" }),
    );
    expect(res.status).toBe(502);
  });
});

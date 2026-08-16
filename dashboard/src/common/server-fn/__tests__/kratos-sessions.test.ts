import { describe, it, expect, vi, afterEach } from "vitest";
import {
  listSessions,
  revokeSession,
} from "@/common/server-fn/kratos-sessions";

const DASHBOARD = "https://dashboard.bex.co";
const CURRENT_ID = "session-current";

function currentSession() {
  return {
    id: CURRENT_ID,
    active: true,
    authenticated_at: "2026-01-01T00:00:00.000Z",
    identity: { id: "identity-xyz", schema_id: "default", traits: {} },
  };
}

function otherSession(id = "session-other") {
  return {
    id,
    active: true,
    authenticated_at: "2026-01-02T00:00:00.000Z",
    devices: [{ id: "device-1", ip_address: "1.2.3.4", location: "SF, US" }],
  };
}

function mockUpstreams(opts: {
  whoamiOk?: boolean;
  otherSessions?: unknown[];
  listOk?: boolean;
  disableOneOk?: boolean;
  disableOthersOk?: boolean;
}) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url.includes("/sessions/whoami")) {
        return new Response(JSON.stringify(currentSession()), {
          status: opts.whoamiOk === false ? 401 : 200,
        });
      }
      const method = init?.method ?? "GET";
      if (url.endsWith("/sessions") && method === "GET") {
        return new Response(
          JSON.stringify(opts.otherSessions ?? [otherSession()]),
          { status: opts.listOk === false ? 500 : 200 },
        );
      }
      if (url.endsWith("/sessions") && method === "DELETE") {
        return new Response(JSON.stringify({ count: 3 }), {
          status: opts.disableOthersOk === false ? 500 : 200,
        });
      }
      if (/\/sessions\/[^/]+$/.test(url) && method === "DELETE") {
        return new Response(null, {
          status: opts.disableOneOk === false ? 500 : 204,
        });
      }
      return new Response("{}", { status: 404 });
    }),
  );
  return calls;
}

const req = (cookie: string | null = "ory_session=live") =>
  new Request(`${DASHBOARD}/api/sessions`, {
    headers: cookie ? { cookie } : {},
  });

const postReq = (
  body: Record<string, unknown>,
  opts: { origin?: string | null; cookie?: string | null } = {},
) => {
  const origin = opts.origin === undefined ? DASHBOARD : opts.origin;
  const cookie = opts.cookie === undefined ? "ory_session=live" : opts.cookie;
  const headers: Record<string, string> = {
    "content-type": "application/json",
  };
  if (origin) headers.origin = origin;
  if (cookie) headers.cookie = cookie;
  return new Request(`${DASHBOARD}/api/sessions`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("listSessions", () => {
  it("returns an empty list with no session", async () => {
    mockUpstreams({ whoamiOk: false });
    const res = await listSessions(req(null));
    expect(await res.json()).toEqual([]);
  });

  it("lists the current session first, marked current, then every other session", async () => {
    mockUpstreams({});
    const res = await listSessions(req());
    const body = await res.json();
    expect(body).toEqual([
      {
        id: CURRENT_ID,
        current: true,
        ipAddress: undefined,
        location: undefined,
        userAgent: undefined,
        authenticatedAt: "2026-01-01T00:00:00.000Z",
      },
      {
        id: "session-other",
        current: false,
        ipAddress: "1.2.3.4",
        location: "SF, US",
        userAgent: undefined,
        authenticatedAt: "2026-01-02T00:00:00.000Z",
      },
    ]);
  });

  it("502s when the list call fails upstream", async () => {
    mockUpstreams({ listOk: false });
    const res = await listSessions(req());
    expect(res.status).toBe(502);
  });
});

describe("revokeSession", () => {
  it("refuses a cross-site request", async () => {
    mockUpstreams({});
    const res = await revokeSession(
      postReq(
        { action: "revoke", id: "session-other" },
        { origin: "https://evil.example" },
      ),
    );
    expect(res.status).toBe(403);
  });

  it("refuses with no session", async () => {
    mockUpstreams({ whoamiOk: false });
    const res = await revokeSession(
      postReq({ action: "revoke", id: "session-other" }),
    );
    expect(res.status).toBe(401);
  });

  it("revokes one specific other session", async () => {
    const calls = mockUpstreams({});
    const res = await revokeSession(
      postReq({ action: "revoke", id: "session-other" }),
    );
    expect(res.status).toBe(204);
    expect(
      calls.some(
        (c) =>
          c.url.endsWith("/sessions/session-other") &&
          c.init?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("signs out every other session", async () => {
    const calls = mockUpstreams({});
    const res = await revokeSession(postReq({ action: "sign-out-others" }));
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ count: 3 });
    expect(
      calls.some(
        (c) => c.url.endsWith("/sessions") && c.init?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("400s on a malformed action", async () => {
    mockUpstreams({});
    const res = await revokeSession(postReq({ action: "bogus" }));
    expect(res.status).toBe(400);
  });
});

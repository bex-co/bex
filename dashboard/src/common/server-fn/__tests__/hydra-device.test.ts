import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// fetchSession is mocked so the device handler's session check is deterministic:
// by default no session (the signed-out login-bounce path the phish-regression
// cases exercise), overridden per-test for the confirmation gate
// (codex-security #9).
const fetchSessionMock = vi.fn();
vi.mock("@/common/server-fn/session", () => ({
  fetchSession: (...args: unknown[]) => fetchSessionMock(...args),
}));

import {
  DESKTOP_CLIENT_ID,
  handleDeviceConfirm,
  handleDeviceVerification,
  RENDER_CLI_CLIENT_ID,
} from "@/common/server-fn/hydra-device";

const DASHBOARD = "https://dashboard.bex.co";
const ADMIN = "http://hydra-admin.test:4445";
const PUBLIC = "https://oauth.bex.co";

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
  process.env.HYDRA_PUBLIC_URL = PUBLIC;
  fetchSessionMock.mockReset();
  fetchSessionMock.mockResolvedValue({ session: null }); // unauthenticated by default
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.HYDRA_ADMIN_URL;
  delete process.env.HYDRA_PUBLIC_URL;
});

describe("handleDeviceVerification", () => {
  it("starts at Hydra's public verifier with the CLI user code", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const result = await handleDeviceVerification(
      new Request(`${DASHBOARD}/auth/device?user_code=ABCDEF`),
    );
    expect(result).toBeInstanceOf(Response);
    const response = result as Response;
    expect(response.status).toBe(302);
    expect(response.headers.get("location")).toBe(
      `${PUBLIC}/oauth2/device/verify?user_code=ABCDEF`,
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("answers honestly when device authorization is not configured", async () => {
    delete process.env.HYDRA_PUBLIC_URL;
    const unconfigured = await handleDeviceVerification(
      new Request(`${DASHBOARD}/auth/device?user_code=ABCDEF`),
    );
    // Renders inside AuthPageShell (w10/m8 t001) rather than a bare Response —
    // the route's GET handler turns this into loader context for the confirm
    // page's error branch.
    expect(unconfigured).not.toBeInstanceOf(Response);
    expect(unconfigured).toEqual({ errorCode: "unconfigured" });
  });

  it("answers honestly when the device code is missing", async () => {
    const missing = await handleDeviceVerification(
      new Request(`${DASHBOARD}/auth/device`),
    );
    expect(missing).not.toBeInstanceOf(Response);
    expect(missing).toEqual({ errorCode: "missing_code" });
  });

  it("renders a recovered error code from a POST-failure redirect, ignoring other params", async () => {
    const result = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?device_error=invalid_code&user_code=STALE`,
      ),
    );
    expect(result).toEqual({ errorCode: "invalid_code" });
  });

  it("bounces a signed-out visitor through login, preserving the code + challenge in `next`, without pairing (device-code phish)", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const result = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-1`,
      ),
    );
    expect(result).toBeInstanceOf(Response);
    const response = result as Response;
    expect(response.status).toBe(302);
    const location = new URL(
      String(response.headers.get("location")),
      DASHBOARD,
    );
    expect(location.origin).toBe(DASHBOARD);
    expect(location.pathname).toBe("/auth/login");
    // The login page returns the browser to this very verification URL after
    // authenticating — where the now-signed-in visitor gets the confirmation
    // page. The grant is NOT paired here: no admin accept call, so the trusted
    // CLI client's skip_consent auto-accept can never be reached from a GET.
    expect(location.searchParams.get("next")).toBe(
      "/auth/device?user_code=ABCDEF&device_challenge=challenge-1",
    );
    expect(location.searchParams.get("aal")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("asks for the second factor when the visitor owes one, instead of looping", async () => {
    fetchSessionMock.mockResolvedValue({
      session: null,
      aal2Required: true,
    });
    const result = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-1`,
      ),
    );
    expect(result).toBeInstanceOf(Response);
    const response = result as Response;
    expect(response.status).toBe(302);
    const location = new URL(
      String(response.headers.get("location")),
      DASHBOARD,
    );
    expect(location.pathname).toBe("/auth/login");
    expect(location.searchParams.get("aal")).toBe("aal2");
    expect(location.searchParams.get("next")).toBe(
      "/auth/device?user_code=ABCDEF&device_challenge=challenge-1",
    );
  });

  it("returns a DeviceView for a signed-in caller instead of pairing silently (codex-security #9)", async () => {
    fetchSessionMock.mockResolvedValue({
      session: { id: "session-abc", active: true, identity: { id: "id-1" } },
    });
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const result = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-1`,
      ),
    );
    // A DeviceView, not a Response: the route hands this to the confirm page
    // (AuthPageShell chrome) as deferred loader context — see auth.device.tsx.
    expect(result).not.toBeInstanceOf(Response);
    expect(result).toEqual({ userCode: "ABCDEF", challenge: "challenge-1" });
    // The grant is NOT paired on the silent GET — no admin accept call.
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("handleDeviceConfirm", () => {
  it("pairs the grant after a same-origin, session-bound POST", async () => {
    fetchSessionMock.mockResolvedValue({
      session: { id: "session-abc", active: true, identity: { id: "id-1" } },
    });
    const calls: { url: string; init?: RequestInit }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: URL | RequestInfo, init?: RequestInit) => {
        calls.push({ url: String(input), init });
        return new Response(
          JSON.stringify({
            redirect_to: `${PUBLIC}/oauth2/device/verify?device_verifier=v&client_id=${RENDER_CLI_CLIENT_ID}`,
          }),
          { status: 200 },
        );
      }),
    );
    const form = new FormData();
    form.set("user_code", "ABCDEF");
    form.set("device_challenge", "challenge-1");
    const response = await handleDeviceConfirm(
      new Request(`${DASHBOARD}/auth/device`, {
        method: "POST",
        body: form,
        headers: { origin: DASHBOARD },
      }),
    );
    expect(response.status).toBe(302);
    expect(response.headers.get("location")).toContain("device_verifier=v");
    expect(calls).toHaveLength(1);
    expect(calls[0].url).toContain(
      "/admin/oauth2/auth/requests/device/accept?device_challenge=challenge-1",
    );
    expect(calls[0].init?.method).toBe("PUT");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({
      user_code: "ABCDEF",
    });
  });

  it("pairs the grant for the first-party bex-desktop client", async () => {
    fetchSessionMock.mockResolvedValue({
      session: { id: "session-abc", active: true, identity: { id: "id-1" } },
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        Response.json({
          redirect_to: `${PUBLIC}/oauth2/device/verify?device_verifier=v&client_id=${DESKTOP_CLIENT_ID}`,
        }),
      ),
    );
    const form = new FormData();
    form.set("user_code", "ABCDEF");
    form.set("device_challenge", "challenge-1");
    const response = await handleDeviceConfirm(
      new Request(`${DASHBOARD}/auth/device`, {
        method: "POST",
        body: form,
        headers: { origin: DASHBOARD },
      }),
    );
    expect(response.status).toBe(302);
    expect(response.headers.get("location")).toContain(
      `client_id=${DESKTOP_CLIENT_ID}`,
    );
  });

  it("fails closed for expired/replayed codes and foreign clients by redirecting into the shell error page (w10/m8 t001)", async () => {
    fetchSessionMock.mockResolvedValue({
      session: { id: "session-abc", active: true, identity: { id: "id-1" } },
    });
    const confirm = () => {
      const form = new FormData();
      form.set("user_code", "ABCDEF");
      form.set("device_challenge", "challenge-1");
      return handleDeviceConfirm(
        new Request(`${DASHBOARD}/auth/device`, {
          method: "POST",
          body: form,
          headers: { origin: DASHBOARD },
        }),
      );
    };

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("expired", { status: 404 })),
    );
    const expired = await confirm();
    expect(expired.status).toBe(303);
    expect(new URL(expired.headers.get("location")!).searchParams.get(
      "device_error",
    )).toBe("invalid_code");

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        Response.json({
          redirect_to: `${PUBLIC}/oauth2/device/verify?device_verifier=v&client_id=foreign`,
        }),
      ),
    );
    const foreign = await confirm();
    expect(foreign.status).toBe(303);
    expect(new URL(foreign.headers.get("location")!).searchParams.get(
      "device_error",
    )).toBe("unexpected_client");
  });

  it("still answers 502 for a genuinely down Hydra (not a recoverable/user-facing state)", async () => {
    fetchSessionMock.mockResolvedValue({
      session: { id: "session-abc", active: true, identity: { id: "id-1" } },
    });
    const form = new FormData();
    form.set("user_code", "ABCDEF");
    form.set("device_challenge", "challenge-1");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Promise.reject(new Error("down"))),
    );
    const response = await handleDeviceConfirm(
      new Request(`${DASHBOARD}/auth/device`, {
        method: "POST",
        body: form,
        headers: { origin: DASHBOARD },
      }),
    );
    expect(response.status).toBe(502);
  });

  it("refuses a cross-site or session-less confirmation", async () => {
    fetchSessionMock.mockResolvedValue({ session: null });
    const form = new FormData();
    form.set("user_code", "ABCDEF");
    form.set("device_challenge", "challenge-1");
    const noSession = await handleDeviceConfirm(
      new Request(`${DASHBOARD}/auth/device`, {
        method: "POST",
        body: form,
        headers: { origin: DASHBOARD },
      }),
    );
    expect(noSession.status).toBe(403);
  });
});

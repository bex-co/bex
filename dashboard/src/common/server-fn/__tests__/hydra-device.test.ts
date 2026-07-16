import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  handleDeviceVerification,
  RENDER_CLI_CLIENT_ID,
} from "@/common/server-fn/hydra-device";

const DASHBOARD = "https://dashboard.bex.co";
const ADMIN = "http://hydra-admin.test:4445";
const PUBLIC = "https://oauth.bex.co";

beforeEach(() => {
  process.env.HYDRA_ADMIN_URL = ADMIN;
  process.env.HYDRA_PUBLIC_URL = PUBLIC;
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
    const response = await handleDeviceVerification(
      new Request(`${DASHBOARD}/auth/device?user_code=ABCDEF`),
    );
    expect(response.status).toBe(302);
    expect(response.headers.get("location")).toBe(
      `${PUBLIC}/oauth2/device/verify?user_code=ABCDEF`,
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("accepts Hydra's challenge server-side and follows only the fixed client", async () => {
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
    const response = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-1`,
      ),
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

  it("fails closed for expired/replayed codes and foreign clients", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("expired", { status: 404 })),
    );
    const expired = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=OLD&device_challenge=stale`,
      ),
    );
    expect(expired.status).toBe(400);

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        Response.json({
          redirect_to: `${PUBLIC}/oauth2/device/verify?device_verifier=v&client_id=foreign`,
        }),
      ),
    );
    const foreign = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-2`,
      ),
    );
    expect(foreign.status).toBe(403);
  });

  it("answers honestly when configuration or Hydra is unavailable", async () => {
    delete process.env.HYDRA_PUBLIC_URL;
    const unconfigured = await handleDeviceVerification(
      new Request(`${DASHBOARD}/auth/device?user_code=ABCDEF`),
    );
    expect(unconfigured.status).toBe(503);

    process.env.HYDRA_PUBLIC_URL = PUBLIC;
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Promise.reject(new Error("down"))),
    );
    const unavailable = await handleDeviceVerification(
      new Request(
        `${DASHBOARD}/auth/device?user_code=ABCDEF&device_challenge=challenge-3`,
      ),
    );
    expect(unavailable.status).toBe(502);
  });
});

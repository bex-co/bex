import { beforeEach, describe, expect, it, vi } from "vitest";
import { reportSignOut, SIGN_OUT_REPORT_TIMEOUT_MS } from "../report-sign-out";

vi.mock("@/config/config", () => ({
  config: { apiUrl: "https://api.example/graphql" },
}));

describe("reportSignOut (LogoutEvent, w9/m452)", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  it("sends recordSignOut to bex-api with the session cookie and a timeout", async () => {
    vi.mocked(fetch).mockResolvedValue(
      Response.json({ data: { recordSignOut: true } }),
    );
    const timeout = vi.spyOn(AbortSignal, "timeout");

    await reportSignOut();

    expect(fetch).toHaveBeenCalledOnce();
    const [url, init] = vi.mocked(fetch).mock.calls[0];
    expect(url).toBe("https://api.example/graphql");
    expect(init).toMatchObject({ method: "POST", credentials: "include" });
    expect(JSON.parse(String(init?.body))).toEqual({
      query: "mutation RecordSignOut { recordSignOut }",
    });
    expect(timeout).toHaveBeenCalledWith(SIGN_OUT_REPORT_TIMEOUT_MS);
    expect(init?.signal).toBeInstanceOf(AbortSignal);
  });

  it.each([
    [
      "a network failure",
      () => Promise.reject(new TypeError("Failed to fetch")),
    ],
    [
      "the timeout",
      () => Promise.reject(new DOMException("timed out", "TimeoutError")),
    ],
    ["a refusal", () => Promise.resolve(new Response(null, { status: 503 }))],
  ])("never throws on %s", async (_, answer) => {
    vi.mocked(fetch).mockImplementation(answer);

    await expect(reportSignOut()).resolves.toBeUndefined();
  });
});

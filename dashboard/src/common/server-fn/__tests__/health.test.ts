import { describe, it, expect, vi, beforeEach } from "vitest";
import { readinessCheck, resetHealthLatchForTests } from "../health";

vi.mock("@/config/config", () => ({
  config: { ssrApiUrl: "http://bex-api.test:8090/graphql" },
}));

beforeEach(() => {
  resetHealthLatchForTests();
});

describe("readinessCheck (w1/m52 t002, folds inbox 036)", () => {
  it("503s while bex-api is unreachable, 200 once it is", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockResolvedValue(new Response("ok", { status: 200 }));

    const before = await readinessCheck(fetchImpl);
    expect(before.status).toBe(503);

    const after = await readinessCheck(fetchImpl);
    expect(after.status).toBe(200);
    // The probe targets bex-api's open /healthz next to the SSR /graphql URL.
    expect(fetchImpl).toHaveBeenCalledWith(
      "http://bex-api.test:8090/healthz",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("treats a non-2xx upstream answer as not ready", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response("no", { status: 503 }));
    expect((await readinessCheck(fetchImpl)).status).toBe(503);
  });

  it("LATCHES: once ready, a later upstream outage does not flip readiness", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response("ok", { status: 200 }))
      .mockRejectedValue(new TypeError("bex-api is down"));

    expect((await readinessCheck(fetchImpl)).status).toBe(200);
    // A steady-state bex-api outage must degrade to inline error states, not
    // cascade into every dashboard pod going NotReady (a total LB 503).
    expect((await readinessCheck(fetchImpl)).status).toBe(200);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});

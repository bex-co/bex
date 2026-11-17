import { describe, expect, it } from "vitest";
import { hookWindowEnd } from "../hook-window";

// w33/m435: the Blueprint's one-time initialDeployHook runs after the deploy
// is live, so the deploy's log does not end at `finishedAt` any more.
describe("hookWindowEnd", () => {
  const live = "2026-11-16T09:00:00.250Z";

  it("is the deploy's own end when no hook ran for it", () => {
    expect(
      hookWindowEnd({
        finishedAt: live,
        initialDeployHookStatus: "",
        initialDeployHookFinishedAt: null,
      }),
    ).toBe(live);
  });

  it("stays open while the hook runs, so the panel keeps polling", () => {
    expect(
      hookWindowEnd({
        finishedAt: live,
        initialDeployHookStatus: "running",
        initialDeployHookFinishedAt: null,
      }),
    ).toBeUndefined();
  });

  it("closes a second past the hook's end, where bex-api writes its closing lines", () => {
    for (const status of ["succeeded", "failed"]) {
      expect(
        hookWindowEnd({
          finishedAt: live,
          initialDeployHookStatus: status,
          initialDeployHookFinishedAt: "2026-11-16T09:00:08Z",
        }),
      ).toBe("2026-11-16T09:00:09.000Z");
    }
  });

  it("never ends before the deploy itself did", () => {
    expect(
      hookWindowEnd({
        finishedAt: "2026-11-16T09:01:00.000Z",
        initialDeployHookStatus: "succeeded",
        initialDeployHookFinishedAt: "2026-11-16T09:00:08Z",
      }),
    ).toBe("2026-11-16T09:01:00.000Z");
  });

  it("is unknown until the deploy has loaded, and open while it is not finished", () => {
    expect(hookWindowEnd(null)).toBeUndefined();
    expect(
      hookWindowEnd({
        finishedAt: null,
        initialDeployHookStatus: "",
        initialDeployHookFinishedAt: null,
      }),
    ).toBeUndefined();
  });

  it("ignores an end time it cannot read", () => {
    expect(
      hookWindowEnd({
        finishedAt: live,
        initialDeployHookStatus: "failed",
        initialDeployHookFinishedAt: "not a time",
      }),
    ).toBe(live);
  });
});

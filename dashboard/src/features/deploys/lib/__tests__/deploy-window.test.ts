import { describe, expect, it } from "vitest";
import { deployWindowEnd } from "../deploy-window";

// A deploy's log runs past `finishedAt` once it went live: bex-api writes
// Render's primary URL block on the six milliseconds after the live line
// (w32/m567), and the Blueprint's one-time initialDeployHook runs after that
// (w33/m435).
describe("deployWindowEnd", () => {
  // bex-api reads `finishedAt` from Postgres in microseconds.
  const live = "2026-11-16T09:00:00.250123Z";
  const deploy = (
    status: string,
    finishedAt: string | null,
    hook: { status: string; finishedAt: string | null } = {
      status: "",
      finishedAt: null,
    },
  ) => ({
    status,
    finishedAt,
    initialDeployHookStatus: hook.status,
    initialDeployHookFinishedAt: hook.finishedAt,
  });

  it("takes in the primary URL block after a live deploy's live line", () => {
    // The block's last line is at .256123; the bound is past it.
    for (const status of ["live", "deactivated"]) {
      expect(deployWindowEnd(deploy(status, live))).toBe(
        "2026-11-16T09:00:00.257Z",
      );
    }
  });

  it("is the deploy's own end when it did not go live", () => {
    for (const status of ["update_failed", "build_failed", "canceled"]) {
      expect(deployWindowEnd(deploy(status, live))).toBe(live);
    }
  });

  it("stays open while the hook runs, so the panel keeps polling", () => {
    expect(
      deployWindowEnd(
        deploy("live", live, { status: "running", finishedAt: null }),
      ),
    ).toBeUndefined();
  });

  it("closes a second past the hook's end, where bex-api writes its closing lines", () => {
    for (const status of ["succeeded", "failed"]) {
      expect(
        deployWindowEnd(
          deploy("live", live, { status, finishedAt: "2026-11-16T09:00:08Z" }),
        ),
      ).toBe("2026-11-16T09:00:09.000Z");
    }
  });

  it("never ends before the deploy's own lines did", () => {
    expect(
      deployWindowEnd(
        deploy("live", "2026-11-16T09:01:00.000Z", {
          status: "succeeded",
          finishedAt: "2026-11-16T09:00:08Z",
        }),
      ),
    ).toBe("2026-11-16T09:01:00.007Z");
  });

  it("is unknown until the deploy has loaded, and open while it is not finished", () => {
    expect(deployWindowEnd(null)).toBeUndefined();
    expect(deployWindowEnd(deploy("update_in_progress", null))).toBeUndefined();
  });

  it("ignores a time it cannot read", () => {
    expect(
      deployWindowEnd(
        deploy("live", live, { status: "failed", finishedAt: "not a time" }),
      ),
    ).toBe("2026-11-16T09:00:00.257Z");
    expect(deployWindowEnd(deploy("live", "not a time"))).toBe("not a time");
  });
});

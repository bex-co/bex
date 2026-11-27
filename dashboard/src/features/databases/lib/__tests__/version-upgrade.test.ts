import { describe, expect, it } from "vitest";
import {
  isUpgradeClone,
  unacknowledgedFailure,
  upgradeAvailable,
  upgradeCloneName,
  versionUpgradeEligible,
} from "@/features/databases/lib/version-upgrade";

// w3/m676: the guided page's rules, as Render's VersionUpgrade page applies
// them.

describe("versionUpgradeEligible / upgradeAvailable", () => {
  const paid = { plan: "basic-1gb", versionUpgradeAvailable: "18" };

  it("offers the latest major to a paid database on an older one", () => {
    expect(upgradeAvailable({ ...paid, version: "16" })).toBe(true);
    expect(upgradeAvailable({ ...paid, version: "13" })).toBe(true);
  });

  it("offers nothing on the latest major, or without a known version", () => {
    expect(upgradeAvailable({ ...paid, version: "18" })).toBe(false);
    expect(upgradeAvailable({ ...paid, version: null })).toBe(false);
    expect(
      upgradeAvailable({
        ...paid,
        version: "16",
        versionUpgradeAvailable: null,
      }),
    ).toBe(false);
  });

  it("has no upgrade page for a free database, as on Render", () => {
    expect(versionUpgradeEligible({ plan: "free" })).toBe(false);
    expect(versionUpgradeEligible({ plan: "basic-1gb" })).toBe(true);
    expect(
      upgradeAvailable({
        plan: "free",
        version: "16",
        versionUpgradeAvailable: "18",
      }),
    ).toBe(false);
  });
});

describe("test-upgrade clones", () => {
  it("names a clone <source>-upgrade-clone-<4>, Render's name", () => {
    let i = 0;
    const digits = [0, 0.5, 0.99, 0.1];
    const name = upgradeCloneName("orders", () => digits[i++]);
    expect(name).toBe("orders-upgrade-clone-as9d");
    expect(isUpgradeClone(name)).toBe(true);
  });

  it("knows a clone by its name only", () => {
    expect(isUpgradeClone("orders-upgrade-clone-ab12")).toBe(true);
    expect(isUpgradeClone("orders")).toBe(false);
    expect(isUpgradeClone("orders-upgrade-clone-")).toBe(false);
    expect(isUpgradeClone("orders-upgrade-clone-ab12-copy")).toBe(false);
  });
});

describe("unacknowledgedFailure", () => {
  const run = {
    runId: "pgu-1",
    startedAt: "2026-11-26T10:00:00Z",
    toVersion: "18",
  };
  it("is a failed run nobody dismissed", () => {
    expect(
      unacknowledgedFailure({ ...run, state: "FAILED", acknowledged: false }),
    ).toBe(true);
    expect(
      unacknowledgedFailure({ ...run, state: "FAILED", acknowledged: true }),
    ).toBe(false);
    expect(
      unacknowledgedFailure({
        ...run,
        state: "SUCCEEDED",
        acknowledged: false,
      }),
    ).toBe(false);
    expect(unacknowledgedFailure(null)).toBe(false);
  });
});

import { freshnessSnapshot } from "../freshness";

describe("freshnessSnapshot", () => {
  it("distinguishes unloaded, fresh, and stale data at the exact boundary", () => {
    expect(freshnessSnapshot(null, 1_000, 5_000)).toEqual({
      status: "not-loaded",
      label: "freshness.notLoaded",
      ageMs: null,
      staleAt: null,
    });
    expect(freshnessSnapshot(4_001, 1_000, 5_000)).toEqual({
      status: "fresh",
      label: "freshness.current",
      ageMs: 999,
      staleAt: 5_001,
    });
    expect(freshnessSnapshot(4_000, 1_000, 5_000)).toEqual({
      status: "stale",
      label: "freshness.stale",
      ageMs: 1_000,
      staleAt: 5_000,
    });
  });

  it("does not report negative age after a clock correction", () => {
    expect(freshnessSnapshot(new Date(6_000), 1_000, 5_000).ageMs).toBe(0);
    expect(freshnessSnapshot(Number.NaN, 1_000, 5_000).status).toBe(
      "not-loaded",
    );
  });
});

import { describe, it, expect } from "vitest";
import { RANGE_PRESETS, DEFAULT_RANGE_PRESET, resolveRange } from "../range";

describe("RANGE_PRESETS", () => {
  it("has the three PoC presets in ascending span order", () => {
    expect(RANGE_PRESETS.map((p) => p.id)).toEqual(["30m", "1h", "12h"]);
    for (let i = 1; i < RANGE_PRESETS.length; i++) {
      expect(RANGE_PRESETS[i].spanSeconds).toBeGreaterThan(
        RANGE_PRESETS[i - 1].spanSeconds,
      );
    }
  });

  it("defaults to the 1h preset", () => {
    expect(DEFAULT_RANGE_PRESET.id).toBe("1h");
  });
});

describe("resolveRange", () => {
  it("computes startTime as now minus the preset's span, endTime as now", () => {
    const now = new Date("2026-07-06T12:00:00.000Z");
    const preset = RANGE_PRESETS.find((p) => p.id === "30m")!;

    const { startTime, endTime, resolutionSeconds } = resolveRange(preset, now);

    expect(endTime).toBe("2026-07-06T12:00:00.000Z");
    expect(startTime).toBe("2026-07-06T11:30:00.000Z");
    expect(resolutionSeconds).toBe(preset.resolutionSeconds);
  });

  it("slides forward with the given `now` (a later call gets a later window)", () => {
    const preset = RANGE_PRESETS.find((p) => p.id === "1h")!;
    const earlier = resolveRange(preset, new Date("2026-07-06T12:00:00.000Z"));
    const later = resolveRange(preset, new Date("2026-07-06T12:05:00.000Z"));

    expect(new Date(later.startTime).getTime()).toBeGreaterThan(
      new Date(earlier.startTime).getTime(),
    );
    expect(new Date(later.endTime).getTime()).toBeGreaterThan(
      new Date(earlier.endTime).getTime(),
    );
  });
});

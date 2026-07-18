import { describe, it, expect } from "vitest";
import {
  RANGE_PRESETS,
  DEFAULT_RANGE_PRESET,
  parseRangePreset,
  resolveRange,
} from "../range";

describe("RANGE_PRESETS", () => {
  it("offers Render's preset ladder in ascending span order (w5/m42)", () => {
    expect(RANGE_PRESETS.map((p) => p.id)).toEqual([
      "30m",
      "1h",
      "4h",
      "12h",
      "24h",
      "2d",
      "7d",
      "14d",
    ]);
    for (let i = 1; i < RANGE_PRESETS.length; i++) {
      expect(RANGE_PRESETS[i].spanSeconds).toBeGreaterThan(
        RANGE_PRESETS[i - 1].spanSeconds,
      );
    }
  });

  it("keeps every preset readable: ~120-150 points at its own resolution", () => {
    for (const p of RANGE_PRESETS) {
      const points = p.spanSeconds / p.resolutionSeconds;
      expect(points).toBeGreaterThanOrEqual(60);
      expect(points).toBeLessThanOrEqual(150);
    }
  });

  it("defaults to the 12h preset (Render's own default)", () => {
    expect(DEFAULT_RANGE_PRESET.id).toBe("12h");
  });

  it("parses supported URL values and rejects malformed/retired ones", () => {
    expect(parseRangePreset("4h")?.id).toBe("4h");
    expect(parseRangePreset("14d")?.id).toBe("14d");
    // retired pre-w5/m42 ids and unsupported spans are rejected, not guessed
    expect(parseRangePreset("6h")).toBeNull();
    expect(parseRangePreset("1d")).toBeNull();
    expect(parseRangePreset("30d")).toBeNull();
    expect(parseRangePreset(["1h"])).toBeNull();
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

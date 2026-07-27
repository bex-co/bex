import { describe, it, expect } from "vitest";
import {
  RANGE_PRESETS,
  DEFAULT_RANGE_PRESET,
  MAX_CUSTOM_RANGE_HOURS,
  makeCustomRange,
  parseCustomRange,
  parseRangePreset,
  resolutionForSpan,
  resolveRange,
} from "../range";

describe("RANGE_PRESETS", () => {
  it("offers Render's preset ladder in ascending span order (w5/m42, +30d w5/m56)", () => {
    expect(RANGE_PRESETS.map((p) => p.id)).toEqual([
      "30m",
      "1h",
      "4h",
      "12h",
      "24h",
      "2d",
      "7d",
      "14d",
      "30d",
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
    expect(parseRangePreset("30d")?.id).toBe("30d"); // w5/m56 — now offered
    // retired pre-w5/m42 ids and unsupported spans are rejected, not guessed
    expect(parseRangePreset("6h")).toBeNull();
    expect(parseRangePreset("1d")).toBeNull();
    expect(parseRangePreset("custom")).toBeNull(); // custom is not a preset
    expect(parseRangePreset(["1h"])).toBeNull();
  });
});

describe("custom range (w5/m56)", () => {
  it("builds an absolute window with a span-scaled resolution", () => {
    const made = makeCustomRange(
      "2026-07-01T00:00:00.000Z",
      "2026-07-01T04:00:00.000Z",
    );
    expect(made).toEqual({
      id: "custom",
      startTime: "2026-07-01T00:00:00.000Z",
      endTime: "2026-07-01T04:00:00.000Z",
      resolutionSeconds: resolutionForSpan(4 * 60 * 60),
    });
  });

  it("rejects a non-ascending window and one past the max query window", () => {
    expect(makeCustomRange("2026-07-02T00:00:00Z", "2026-07-01T00:00:00Z")).toBe(
      "order",
    );
    expect(makeCustomRange("bad", "also-bad")).toBe("order");
    const start = new Date("2026-07-01T00:00:00Z");
    const tooLong = new Date(
      start.getTime() + (MAX_CUSTOM_RANGE_HOURS + 1) * 60 * 60 * 1000,
    );
    expect(
      makeCustomRange(start.toISOString(), tooLong.toISOString()),
    ).toBe("tooLong");
  });

  it("round-trips a stored custom window and rejects an invalid one", () => {
    const custom = parseCustomRange(
      "2026-07-01T00:00:00.000Z",
      "2026-07-01T06:00:00.000Z",
    );
    expect(custom?.id).toBe("custom");
    expect(custom?.startTime).toBe("2026-07-01T00:00:00.000Z");
    expect(parseCustomRange("2026-07-02T00:00:00Z", "2026-07-01T00:00:00Z")).toBeNull();
    expect(parseCustomRange(undefined, undefined)).toBeNull();
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

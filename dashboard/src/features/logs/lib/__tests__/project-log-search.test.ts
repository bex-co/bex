import { describe, expect, it } from "vitest";
import {
  parseProjectLogSearch,
  parseProjectMetricsSearch,
  projectLogRange,
  projectLogRangeSearch,
  projectMetricsRange,
  viewTimeRange,
} from "../project-log-search";
import { DEFAULT_LOG_RANGE, LIVE_TAIL } from "../log-range";
import {
  makeCustomRange,
  parseRangePreset,
} from "@/features/metrics/lib/range";

// A project observability view's URL keeps Render's keys (w3/m505):
// `savedSearchId` and `r`, plus the Logs tab's custom-window bounds.
describe("project log view search", () => {
  it("reads Render's savedSearchId and r", () => {
    expect(
      parseProjectLogSearch({ savedSearchId: "svs-1", r: "1h" }),
    ).toStrictEqual({
      savedSearchId: "svs-1",
      r: "1h",
      rangeStart: undefined,
      rangeEnd: undefined,
      contextLine: undefined,
    });
    // Render tokens without a bex preset map onto the nearest one.
    expect(parseProjectLogSearch({ r: "6h" }).r).toBe("4h");
    expect(parseProjectLogSearch({ r: "bogus" }).r).toBeUndefined();
    expect(parseProjectLogSearch({ savedSearchId: 7 }).savedSearchId).toBe("7");
  });

  it("keeps a custom window only with both valid bounds", () => {
    const start = "2026-11-18T00:00:00.000Z";
    const end = "2026-11-18T06:00:00.000Z";
    expect(
      parseProjectLogSearch({ r: "custom", rangeStart: start, rangeEnd: end }),
    ).toMatchObject({ r: "custom", rangeStart: start, rangeEnd: end });
    expect(
      parseProjectLogSearch({ r: "custom", rangeStart: end, rangeEnd: start }),
    ).toMatchObject({ r: undefined, rangeStart: undefined });
  });

  it("round-trips a range through the URL keys", () => {
    const custom = makeCustomRange(
      "2026-11-18T00:00:00.000Z",
      "2026-11-18T06:00:00.000Z",
    );
    if (typeof custom === "string") throw new Error(custom);
    for (const range of [parseRangePreset("24h")!, custom]) {
      const search = parseProjectLogSearch({
        savedSearchId: "svs-1",
        ...projectLogRangeSearch(range),
      });
      expect(projectLogRange(search, DEFAULT_LOG_RANGE)).toEqual(range);
    }
    // A preset clears a custom window's stale bounds, and a View in context
    // mark with them (w3/m528).
    expect(projectLogRangeSearch(parseRangePreset("1h")!)).toStrictEqual({
      r: "1h",
      rangeStart: undefined,
      rangeEnd: undefined,
      contextLine: undefined,
    });
  });

  it("falls back to the view's own range when the URL names none", () => {
    const fallback = parseRangePreset("4h")!;
    expect(projectLogRange({}, fallback)).toBe(fallback);
  });

  it("reads a view's timeRange, or its earlier TIME_RANGE row, as its default range", () => {
    expect(
      viewTimeRange({
        filters: [
          { field: "LEVEL", operator: "IS", values: ["error"] },
          { field: "TIME_RANGE", operator: "IS", values: ["7d"] },
        ],
        version: 1,
      })?.id,
    ).toBe("7d");
    // Render's shape (w3/083).
    expect(
      viewTimeRange({ filters: [], version: 1, timeRange: { relative: "24h" } })
        ?.id,
    ).toBe("24h");
    expect(
      viewTimeRange({
        filters: [],
        version: 1,
        timeRange: { relative: "90d" },
      }),
    ).toBeNull();
    expect(
      viewTimeRange({ filters: [], version: 1, timeRange: {} }),
    ).toBeNull();
    expect(viewTimeRange({ filters: [], version: 1 })).toBeNull();
    expect(
      viewTimeRange({
        filters: [{ field: "TIME_RANGE", operator: "IS", values: ["90d"] }],
      }),
    ).toBeNull();
    expect(viewTimeRange(undefined)).toBeNull();
  });
});

// Live tail is a range (w3/m537), Render's `r=live`.
describe("a project view's Live tail", () => {
  it("reads and writes r=live, and lets a preset leave it", () => {
    const search = parseProjectLogSearch({ savedSearchId: "svs-1", r: "live" });
    expect(search.r).toBe("live");
    expect(projectLogRange(search, DEFAULT_LOG_RANGE)).toEqual(LIVE_TAIL);
    expect(projectLogRangeSearch(LIVE_TAIL)).toStrictEqual({
      r: "live",
      rangeStart: undefined,
      rangeEnd: undefined,
      contextLine: undefined,
    });
  });

  it("opens on the view's saved range, else Last hour, when the URL names none", () => {
    const none = parseProjectLogSearch({ savedSearchId: "svs-1" });
    expect(projectLogRange(none, DEFAULT_LOG_RANGE).id).toBe("1h");
    expect(projectLogRange(none, parseRangePreset("7d")!).id).toBe("7d");
  });

  // Render's Logs never list Metrics' 30m and 12h (w3/100); a Service
  // Metrics view keeps them.
  it("opens a 30m or 12h, from the URL or the view, on the nearest log range", () => {
    const none = parseProjectLogSearch({ savedSearchId: "svs-1" });
    expect(projectLogRange(none, parseRangePreset("12h")!).id).toBe("24h");
    expect(projectLogRange(none, parseRangePreset("30m")!).id).toBe("1h");
    const old = parseProjectLogSearch({ savedSearchId: "svs-1", r: "12h" });
    expect(projectLogRange(old, DEFAULT_LOG_RANGE).id).toBe("24h");
    const render15m = parseProjectLogSearch({ r: "15m" });
    expect(projectLogRange(render15m, DEFAULT_LOG_RANGE).id).toBe("1h");
    const metrics = parseProjectMetricsSearch({ r: "12h" });
    expect(projectMetricsRange(metrics, DEFAULT_LOG_RANGE).id).toBe("12h");
  });

  it("keeps Live tail out of a Service Metrics view", () => {
    const search = parseProjectMetricsSearch({
      savedSearchId: "svs-1",
      r: "live",
    });
    expect(search.r).toBeUndefined();
    expect(
      projectMetricsRange({ ...search, r: "live" }, parseRangePreset("12h")!)
        .id,
    ).toBe("12h");
    expect(parseProjectMetricsSearch({ r: "4h" }).r).toBe("4h");
  });
});

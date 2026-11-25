import { describe, expect, it } from "vitest";
import {
  DEFAULT_LOG_RANGE,
  LIVE_TAIL,
  LIVE_TAIL_HISTORY,
  LOG_RANGE_PRESETS,
  historyRange,
  isPresetRange,
  logRangeEndMs,
  logRangeFromSearch,
  logRangeToSearch,
  metricsRangeSearch,
  parseLogRangeSearch,
  toLogRange,
} from "../log-range";
import {
  RANGE_PRESETS,
  makeCustomRange,
  parseRangePreset,
  parseRangeSearch,
  rangeFromSearch,
} from "@/features/metrics/lib/range";
import {
  datastoreLogSearchProps,
  parseDatastoreLogSearch,
} from "../datastore-log-search";

const custom = makeCustomRange(
  "2026-11-18T00:00:00.000Z",
  "2026-11-18T06:00:00.000Z",
);
if (typeof custom === "string") throw new Error(custom);

// A log explorer's range can be Live tail, as Render's `timeRange` can be
// `live` (w3/m537); Metrics' keys and picker never take it.
describe("the log range", () => {
  it("round-trips Live tail, a preset and a custom window through the range keys", () => {
    for (const range of [LIVE_TAIL, parseRangePreset("24h")!, custom]) {
      const search = parseLogRangeSearch({ ...logRangeToSearch(range) });
      expect(logRangeFromSearch(search, DEFAULT_LOG_RANGE)).toEqual(range);
    }
    // Leaving a custom window for Live tail clears its bounds.
    expect(logRangeToSearch(LIVE_TAIL)).toStrictEqual({
      range: "live",
      rangeStart: undefined,
      rangeEnd: undefined,
    });
  });

  it("falls back for a range it cannot read", () => {
    expect(
      logRangeFromSearch(
        parseLogRangeSearch({ range: "bogus" }),
        DEFAULT_LOG_RANGE,
      ),
    ).toBe(DEFAULT_LOG_RANGE);
    expect(
      logRangeFromSearch(parseLogRangeSearch({ range: "custom" }), LIVE_TAIL),
    ).toBe(LIVE_TAIL);
  });

  it("is refused by Metrics' parser, and stripped before a Metrics read", () => {
    expect(parseRangeSearch({ range: "live" }).range).toBeUndefined();
    const fallback = parseRangePreset("12h")!;
    expect(
      rangeFromSearch(metricsRangeSearch({ range: "live" }), fallback),
    ).toBe(fallback);
    expect(
      rangeFromSearch(metricsRangeSearch({ range: "4h" }), fallback).id,
    ).toBe("4h");
  });

  // Render's `logTimeRangeIntervals` after `live` (w3/100); Metrics' own
  // ladder adds 30m and 12h (range.test.ts pins it).
  it("lists Render's log ranges, Metrics' ladder without 30m and 12h", () => {
    expect(LOG_RANGE_PRESETS.map((preset) => preset.id)).toEqual([
      "1h",
      "4h",
      "24h",
      "2d",
      "7d",
      "14d",
      "30d",
    ]);
    expect(
      RANGE_PRESETS.filter((preset) => !LOG_RANGE_PRESETS.includes(preset)).map(
        (preset) => preset.id,
      ),
    ).toEqual(["30m", "12h"]);
  });

  it("reads Metrics' 30m and 12h as the nearest log range, and every other range as itself", () => {
    expect(toLogRange(parseRangePreset("30m")!)).toBe(parseRangePreset("1h"));
    expect(toLogRange(parseRangePreset("12h")!)).toBe(parseRangePreset("24h"));
    for (const range of [LIVE_TAIL, custom, ...LOG_RANGE_PRESETS]) {
      expect(toLogRange(range)).toBe(range);
    }
    expect(
      logRangeFromSearch(
        parseLogRangeSearch({ range: "12h" }),
        DEFAULT_LOG_RANGE,
      ).id,
    ).toBe("24h");
    expect(
      logRangeFromSearch(
        parseLogRangeSearch({ range: "bogus" }),
        parseRangePreset("30m")!,
      ).id,
    ).toBe("1h");
  });

  it("reads Render's 4-hour history on Live tail, and the range itself otherwise", () => {
    expect(historyRange(LIVE_TAIL)).toBe(LIVE_TAIL_HISTORY);
    expect(LIVE_TAIL_HISTORY.spanSeconds).toBe(4 * 60 * 60);
    expect(historyRange(custom)).toBe(custom);
    expect(isPresetRange(LIVE_TAIL)).toBe(false);
    expect(isPresetRange(custom)).toBe(false);
    expect(isPresetRange(DEFAULT_LOG_RANGE)).toBe(true);
    expect(logRangeEndMs(custom)).toBe(Date.parse(custom.endTime));
    expect(logRangeEndMs(LIVE_TAIL)).toBe(Infinity);
  });
});

describe("a datastore Logs tab's range", () => {
  it("opens on its one-shot Last hour, and on Live tail from range=live", () => {
    const props = (search: Record<string, unknown>) =>
      datastoreLogSearchProps(parseDatastoreLogSearch(search), () => undefined);
    expect(props({}).range.id).toBe("1h");
    expect(props({ range: "live" }).range).toEqual(LIVE_TAIL);
  });

  // A Key Value's Metrics tab shares the keys and keeps 12h; its Logs tab
  // reads it as Last 24 hours (w3/100).
  it("opens a shared range=12h on Last 24 hours while Metrics keeps 12h", () => {
    const search = parseDatastoreLogSearch({ range: "12h" });
    expect(datastoreLogSearchProps(search, () => undefined).range.id).toBe(
      "24h",
    );
    expect(
      rangeFromSearch(metricsRangeSearch(search), DEFAULT_LOG_RANGE).id,
    ).toBe("12h");
  });

  it("writes Live tail as range=live, and drops a View in context mark with it", () => {
    const writes: unknown[] = [];
    datastoreLogSearchProps({ contextLine: "id:a" }, (patch) =>
      writes.push(patch),
    ).onRangeChange(LIVE_TAIL);
    expect(writes).toStrictEqual([
      {
        range: "live",
        rangeStart: undefined,
        rangeEnd: undefined,
        contextLine: undefined,
      },
    ]);
  });
});

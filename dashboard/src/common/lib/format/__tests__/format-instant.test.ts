import { afterEach, describe, expect, it } from "vitest";

import {
  formatDateTimeWithZone,
  formatDuration,
  formatInstantDetails,
  formatInstantGap,
  formatTimeAgo,
} from "../";

const INSTANT = "2026-07-16T00:01:30Z";
const AT = Date.parse(INSTANT);
const opts = { now: AT, justNow: "just now" };

describe("formatTimeAgo", () => {
  it("spells elapsed time out in full, Render's deploy-row style", () => {
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT + 90_000 })).toBe(
      "2 minutes ago",
    );
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT + 3 * 3600_000 })).toBe(
      "3 hours ago",
    );
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT + 27 * 3600_000 })).toBe(
      "1 day ago",
    );
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT + 40 * 86400_000 })).toBe(
      "1 month ago",
    );
  });

  it("uses the caller's 'just now' under a minute, including for clock skew", () => {
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT + 59_000 })).toBe(
      "just now",
    );
    // A server-stamped instant ahead of the viewer's clock must never read
    // "in 5 seconds".
    expect(formatTimeAgo(INSTANT, { ...opts, now: AT - 5_000 })).toBe(
      "just now",
    );
  });

  it("follows the active language for the unit words", () => {
    expect(
      formatTimeAgo(INSTANT, {
        ...opts,
        now: AT + 3 * 3600_000,
        language: "zh",
      }),
    ).toBe("3 小时前");
    expect(
      formatTimeAgo(INSTANT, {
        ...opts,
        now: AT + 3 * 3600_000,
        language: "en",
      }),
    ).toBe("3 hours ago");
  });

  // The other languages word date-fns' unit choice through Intl, so the bundle
  // carries no date-fns locale for them (language.ts, intlDistanceLocale).
  it.each([
    ["ja", "3 時間前"],
    ["ko", "3시간 전"],
    ["es", "hace 3 horas"],
    ["pt", "há 3 horas"],
    ["ru", "3 часа назад"],
    ["de", "vor 3 Stunden"],
  ])("words the unit natively in %s", (language, expected) => {
    expect(
      formatTimeAgo(INSTANT, { ...opts, now: AT + 3 * 3600_000, language }),
    ).toBe(expected);
  });

  it("keeps date-fns' strict unit choice and rounding in every language", () => {
    // 40 days is "1 month", exactly as en reads it — date-fns still decides.
    expect(
      formatTimeAgo(INSTANT, {
        ...opts,
        now: AT + 40 * 86400_000,
        language: "de",
      }),
    ).toBe("vor 1 Monat");
    expect(
      formatTimeAgo(INSTANT, { ...opts, now: AT + 90_000, language: "ru" }),
    ).toBe("2 минуты назад");
  });

  it("returns null for a missing or unparseable instant", () => {
    expect(formatTimeAgo(null, opts)).toBeNull();
    expect(formatTimeAgo("", opts)).toBeNull();
    expect(formatTimeAgo("not-a-date", opts)).toBeNull();
  });
});

// Render's cron job "Last successful run" (`LL [at] LT z`, w3/m643): the
// dashboard's date and time in the runtime's zone, then the zone's short
// name in the dashboard language.
describe("formatDateTimeWithZone", () => {
  const suiteZone = process.env.TZ;
  afterEach(() => {
    process.env.TZ = suiteZone;
  });

  it("names UTC on UTC, the suite's zone", () => {
    expect(formatDateTimeWithZone("2026-11-21T17:00:00Z", "en")).toBe(
      "November 21, 2026 at 5:00 PM UTC",
    );
  });

  it("reads a viewer's own zone and names it", () => {
    process.env.TZ = "America/Los_Angeles";
    expect(formatDateTimeWithZone("2026-11-21T17:00:00Z", "en")).toBe(
      "November 21, 2026 at 9:00 AM PST",
    );
    expect(formatDateTimeWithZone("2026-07-16T16:00:00Z", "en")).toBe(
      "July 16, 2026 at 9:00 AM PDT",
    );
    process.env.TZ = "Asia/Shanghai";
    expect(formatDateTimeWithZone("2026-11-21T17:00:00Z", "zh")).toBe(
      "2026年11月22日 上午1:00 GMT+8",
    );
  });

  it("returns null for a missing or unparseable instant", () => {
    expect(formatDateTimeWithZone(null, "en")).toBeNull();
    expect(formatDateTimeWithZone("not-a-date", "en")).toBeNull();
  });
});

// Render's time card (`TimestampTooltipContent`, w4/716). vitest runs with
// TZ=UTC; a zone is passed to read the card as a viewer elsewhere would.
describe("formatInstantDetails", () => {
  it("reads Render's rows for a viewer in UTC, with no second UTC row", () => {
    expect(formatInstantDetails(INSTANT, "en")).toEqual({
      zone: "UTC",
      local: "Jul 16 12:01:30 AM",
      localCopy: "2026-07-16T00:01:30.000+00:00",
      utc: null,
      utcCopy: "2026-07-16T00:01:30.000Z",
      unix: "1784160090000",
    });
  });

  it("names the viewer's zone and adds the UTC row outside UTC", () => {
    expect(
      formatInstantDetails(
        "2026-07-16T00:01:30.042Z",
        "en",
        "America/Los_Angeles",
      ),
    ).toEqual({
      zone: "PDT",
      local: "Jul 15 05:01:30 PM",
      localCopy: "2026-07-15T17:01:30.042-07:00",
      utc: "Jul 16 12:01:30 AM",
      utcCopy: "2026-07-16T00:01:30.042Z",
      unix: "1784160090042",
    });
  });

  it("copies a half-hour offset, and a zone with no abbreviation by its offset", () => {
    const details = formatInstantDetails(INSTANT, "en", "Asia/Kolkata")!;
    expect(details.zone).toBe("GMT+5:30");
    expect(details.local).toBe("Jul 16 05:31:30 AM");
    expect(details.localCopy).toBe("2026-07-16T05:31:30.000+05:30");
  });

  it("keeps the line's milliseconds, dropping finer digits", () => {
    expect(
      formatInstantDetails("2026-11-19T10:36:01.709123456Z", "en")?.unix,
    ).toBe("1795084561709");
  });

  it("returns null for a missing or unparseable instant", () => {
    expect(formatInstantDetails(null)).toBeNull();
    expect(formatInstantDetails(undefined)).toBeNull();
    expect(formatInstantDetails("garbage")).toBeNull();
  });
});

describe("formatInstantGap", () => {
  it("is the time since the earlier instant, in Render's duration format", () => {
    expect(formatInstantGap(INSTANT, "2026-07-16T00:00:20Z")).toBe("1m 10s");
    expect(formatInstantGap(INSTANT, INSTANT)).toBe("0ms");
  });

  it("reads a negative gap with a minus, as Render's does", () => {
    expect(formatInstantGap("2026-07-16T00:01:29.958Z", INSTANT)).toBe("-42ms");
  });

  it("is null when either instant is missing", () => {
    expect(formatInstantGap(INSTANT, "")).toBeNull();
    expect(formatInstantGap("", INSTANT)).toBeNull();
    expect(formatInstantGap(INSTANT, "garbage")).toBeNull();
  });
});

// Render's duration format (`utils-*.js`), shared by a cron run's duration
// and a log line's From previous and From start (w4/716).
describe("formatDuration", () => {
  it.each([
    [0, "0ms"],
    [42, "42ms"],
    [99, "99ms"],
    [99.4, "99ms"],
    [99.5, "0.1s"],
    [100, "0.1s"],
    [999, "1s"],
    [1_200, "1.2s"],
    [14_500, "14.5s"],
    [59_900, "59.9s"],
    [59_940, "59.9s"],
    [59_950, "1m"],
    [60_000, "1m"],
    [65_000, "1m 5s"],
    [184_000, "3m 4s"],
    [3_599_000, "59m 59s"],
    [3_600_000, "1h"],
    [3_723_000, "1h 2m"],
    [7_500_000, "2h 5m"],
    [86_399_000, "24h"],
    [86_400_000, "1d"],
    [90_000_000, "1d 1h"],
    [97_200_000, "1d 3h"],
  ])("%d ms reads %s", (ms, want) => {
    expect(formatDuration(ms, "en")).toBe(want);
  });

  it("reads a fraction of a millisecond to one digit", () => {
    expect(formatDuration(0.04, "en")).toBe("0.04ms");
    expect(formatDuration(0.5, "en")).toBe("0.5ms");
  });

  it("writes the decimal the dashboard's language writes, keeping Render's units", () => {
    expect(formatDuration(14_500, "de")).toBe("14,5s");
    expect(formatDuration(14_500, "zh")).toBe("14.5s");
    expect(formatDuration(-1_200, "en")).toBe("-1.2s");
  });
});

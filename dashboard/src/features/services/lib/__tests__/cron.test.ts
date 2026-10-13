import { describe, expect, it } from "vitest";

import { readRepoVectors } from "@/test/repo-vectors";

import {
  cronScheduleProblem,
  describeCron,
  isValidCron,
} from "@/features/services/lib/cron";
import { translator } from "@/test/translator";

const t = translator("en");

// The one acceptance table for a cron schedule. bex-api's checkCronSchedule is
// tested against the same file (lego/backend/internal/apps/
// cron_schedule_vectors_test.go), and its answers are the source of truth: each
// row's code is bex-api's refusal code, null for a schedule it accepts. The form
// must accept exactly what the server accepts (w1/m145) and refuse it for the
// same reason (w5/m118).
const VECTORS = readRepoVectors<{ schedule: string; code: string | null }>(
  "lego/backend/internal/apps/testdata/cron-schedule-vectors.json",
);

// The problem cronScheduleProblem names for each of bex-api's refusal codes.
const PROBLEM_FOR_CODE: Record<string, "format" | "never_fires"> = {
  SCHEDULE_INVALID: "format",
  SCHEDULE_NEVER_FIRES: "never_fires",
};

describe("the shared cron vectors", () => {
  it.each(VECTORS.map((v) => [v.schedule, v.code] as const))(
    "cronScheduleProblem(%j) answers bex-api's %s",
    (schedule, code) => {
      const want = code === null ? null : PROBLEM_FOR_CODE[code];
      expect(want, `unmapped bex-api code ${code}`).not.toBeUndefined();
      expect(cronScheduleProblem(schedule)).toBe(want);
      expect(isValidCron(schedule)).toBe(code === null);
    },
  );

  it("is checked against a non-trivial table", () => {
    const count = (code: string | null) =>
      VECTORS.filter((v) => v.code === code).length;
    expect(count(null)).toBeGreaterThanOrEqual(10);
    expect(count("SCHEDULE_INVALID")).toBeGreaterThanOrEqual(10);
    expect(count("SCHEDULE_NEVER_FIRES")).toBeGreaterThanOrEqual(5);
  });
});

// The vector table above already runs every row through cronScheduleProblem;
// these pin which refusal each draft gets, which the
// settings editor computes on every keystroke (w5/070: classifying "*/0 * * * *"
// used to expand the zero step forever).
describe("cronScheduleProblem", () => {
  it("calls a schedule robfig cannot parse a format error, never a never-firing one", () => {
    for (const schedule of [
      "*/0 * * * *",
      "0 0 1-99999999 * *",
      "9007199254740993 * * * *",
      "* * 32 * *",
      "* * * 13 *",
      "5-2 * * * *",
      "0 0 99 2 *",
      "* * *",
      "",
    ]) {
      expect(cronScheduleProblem(schedule), schedule).toBe("format");
    }
  });

  it("calls a schedule that parses but has no date to fire on never_fires", () => {
    for (const schedule of [
      "0 0 31 2 *",
      "0 0 30,31 2 *",
      "0 0 31 2,4,6,9,11 *",
      "0 0 31 2 ?",
      "0 0 31 2 */1",
      ", * * * *",
    ]) {
      expect(cronScheduleProblem(schedule), schedule).toBe("never_fires");
    }
  });
});

describe("describeCron", () => {
  it("names the common shapes", () => {
    expect(describeCron("* * * * *", t)).toBe("Every minute");
    expect(describeCron("*/5 * * * *", t)).toBe("Every 5 minutes");
    expect(describeCron("0 * * * *", t)).toBe("Every hour");
    expect(describeCron("30 * * * *", t)).toBe("Every hour at minute 30");
    expect(describeCron("0 */2 * * *", t)).toBe("Every 2 hours");
    expect(describeCron("0 0 * * *", t)).toBe("Every day at 00:00");
    expect(describeCron("30 9 * * *", t)).toBe("Every day at 09:30");
    expect(describeCron("0 8 * * 1", t)).toBe("Every Monday at 08:00");
    expect(describeCron("0 0 1 * *", t)).toBe(
      "On day 1 of every month at 00:00",
    );
  });

  // w4/277: the preview interpolated numeric fields as typed, so an accepted
  // "0005 0009 * * *" (09:05 UTC to the server's Atoi) read "0009:0005". A
  // padded schedule previews exactly like its unpadded form; acceptance and the
  // saved text are unchanged.
  it("previews zero-padded numbers like their unpadded form", () => {
    const zh = translator("zh");
    const pairs: [string, string][] = [
      ["0005 0009 * * *", "5 9 * * *"],
      ["05 09 * * *", "5 9 * * *"],
      ["0000 * * * *", "0 * * * *"],
      ["007 * * * *", "7 * * * *"],
      ["00 008 * * 0001", "0 8 * * 1"],
      ["0005 0009 001 * *", "5 9 1 * *"],
      ["*/05 * * * *", "*/5 * * * *"],
      ["0 */02 * * *", "0 */2 * * *"],
      ["00000000000000000000005 * * * *", "5 * * * *"],
    ];
    for (const [padded, plain] of pairs) {
      expect(isValidCron(padded), padded).toBe(true);
      expect(describeCron(padded, t), padded).toBe(describeCron(plain, t));
      expect(describeCron(padded, zh), padded).toBe(describeCron(plain, zh));
    }
    expect(describeCron("0005 0009 * * *", t)).toBe("Every day at 09:05");
    expect(describeCron("0005 0009 001 * *", t)).toBe(
      "On day 1 of every month at 09:05",
    );
    expect(describeCron("0000 * * * *", t)).toBe("Every hour");
    expect(describeCron("0005 0009 * * *", zh)).toBe("每天 09:05");
  });

  it("returns null for invalid input and unhandled shapes", () => {
    expect(describeCron("99 99 * * *", t)).toBeNull();
    expect(describeCron("not a cron", t)).toBeNull();
    expect(describeCron("0 0 * * 1-5", t)).toBeNull();
  });

  // w9/061: both step branches interpolated the captured step into "Every N
  // minutes/hours" without bounding it against the field, so a schedule the
  // backend accepts was previewed as an interval cron never runs. The step only
  // names a uniform interval when it divides the field's width.
  it("does not promise a uniform interval for a non-divisor step", () => {
    // Minutes: fires at :00 and :40, so the gaps alternate 40 and 20.
    expect(describeCron("*/40 * * * *", t)).toBeNull();
    expect(describeCron("*/7 * * * *", t)).toBeNull();
    // Hours: fires at 0,5,10,15,20 — a four-hour gap across midnight.
    expect(describeCron("0 */5 * * *", t)).toBeNull();
    expect(describeCron("0 */7 * * *", t)).toBeNull();
  });

  it("collapses a step wider than its field to the unit it actually runs", () => {
    // Previously "Every 70 minutes" / "Every 100 minutes": only minute 0 fires.
    expect(describeCron("*/70 * * * *", t)).toBe("Every hour");
    expect(describeCron("*/100 * * * *", t)).toBe("Every hour");
    // Previously "Every 30 hours" — an interval cron cannot express at all.
    expect(describeCron("0 */30 * * *", t)).toBe("Every day at 00:00");
    expect(describeCron("15 */30 * * *", t)).toBe("Every day at 00:15");
  });

  it("keeps the exact-fit boundaries truthful", () => {
    // Both fire only at the field's start, and both intervals are exact.
    expect(describeCron("*/60 * * * *", t)).toBe("Every hour");
    expect(describeCron("0 */24 * * *", t)).toBe("Every day at 00:00");
    // Ordinary divisors keep their useful preview.
    expect(describeCron("*/30 * * * *", t)).toBe("Every 30 minutes");
    expect(describeCron("*/15 * * * *", t)).toBe("Every 15 minutes");
    expect(describeCron("0 */12 * * *", t)).toBe("Every 12 hours");
    expect(describeCron("0 */6 * * *", t)).toBe("Every 6 hours");
  });

  // The preview must never disagree with what the server accepts: every
  // schedule above stays valid, so the form still submits each one.
  it("still accepts every schedule it declines to describe", () => {
    for (const s of [
      "*/40 * * * *",
      "*/70 * * * *",
      "*/100 * * * *",
      "0 */5 * * *",
      "0 */30 * * *",
    ]) {
      expect(isValidCron(s)).toBe(true);
    }
  });

  // w6/048: describeCron re-parsed fields with /^\d+$/-only checks, so a named
  // weekday/month that isValidCron happily accepted fell through every phrase
  // branch to null. Named tokens must describe identically to their numeric
  // twins — including the shapes the numeric path itself declines (null).
  describe("named weekday/month tokens (w6/048)", () => {
    it.each([
      // [named, numeric twin]
      ["0 0 * * MON", "0 0 * * 1"],
      ["0 0 * * mon", "0 0 * * 1"],
      ["30 9 * * FRI", "30 9 * * 5"],
      ["0 0 * * SUN", "0 0 * * 0"],
      // month: a named month behaves like its number — no phrase branch names
      // a specific month, so both sides are null, but they must agree.
      ["0 0 5 JAN *", "0 0 5 1 *"],
      ["15 8 1 DEC *", "15 8 1 12 *"],
      // named ranges/lists mirror the numeric path, which declines ranges and
      // lists in dow — both describe as null, never as a wrong phrase.
      ["0 0 * * MON-FRI", "0 0 * * 1-5"],
      ["0 0 * * MON,WED", "0 0 * * 1,3"],
    ] as const)("describes %s identically to %s", (named, numeric) => {
      expect(isValidCron(named)).toBe(true);
      expect(describeCron(named, t)).toBe(describeCron(numeric, t));
    });

    it("produces the numeric path's exact phrases for named single tokens", () => {
      expect(describeCron("0 0 * * MON", t)).toBe("Every Monday at 00:00");
      expect(describeCron("0 0 * * SUN", t)).toBe("Every Sunday at 00:00");
    });
  });

  // w1/m145: 7 is not Sunday to bex-api, so it gets no Sunday preview; ? is
  // robfig's synonym for *, so it previews exactly like *.
  it("previews ? like * and gives 7 no preview", () => {
    expect(describeCron("0 0 * * 7", t)).toBeNull();
    expect(describeCron("0 0 ? * *", t)).toBe("Every day at 00:00");
    expect(describeCron("0 8 ? * 1", t)).toBe("Every Monday at 08:00");
    expect(describeCron("? ? ? ? ?", t)).toBe("Every minute");
  });
});

// w4/m188: the preview phrases and weekday names were English literals, so the
// translated "{description} · 使用 UTC" wrapper interpolated an English
// sentence. Every phrase branch now resolves through the caller's translator.
describe("describeCron in Chinese", () => {
  const zh = translator("zh");

  it("localizes every phrase branch", () => {
    expect(describeCron("* * * * *", zh)).toBe("每分钟");
    expect(describeCron("*/5 * * * *", zh)).toBe("每 5 分钟");
    expect(describeCron("0 * * * *", zh)).toBe("每小时");
    expect(describeCron("*/70 * * * *", zh)).toBe("每小时");
    expect(describeCron("30 * * * *", zh)).toBe("每小时第 30 分钟");
    expect(describeCron("0 */2 * * *", zh)).toBe("每 2 小时");
    expect(describeCron("15 */30 * * *", zh)).toBe("每天 00:15");
    expect(describeCron("30 9 * * *", zh)).toBe("每天 09:30");
    expect(describeCron("0 0 1 * *", zh)).toBe("每月 1 日 00:00");
  });

  it("localizes every weekday, numeric and named alike", () => {
    const days = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
    const names = ["SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"];
    days.forEach((day, i) => {
      expect(describeCron(`0 0 * * ${i}`, zh)).toBe(`每${day} 00:00`);
      expect(describeCron(`0 0 * * ${names[i]}`, zh)).toBe(`每${day} 00:00`);
    });
    expect(describeCron("0 0 * * MON", t)).toBe("Every Monday at 00:00");
  });

  it("keeps the same null fallbacks in every language", () => {
    for (const s of [
      "*/40 * * * *",
      "0 */5 * * *",
      "99 99 * * *",
      "0 0 * * 7",
    ]) {
      expect(describeCron(s, zh)).toBeNull();
    }
  });
});

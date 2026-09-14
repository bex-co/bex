import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { describeCron, isValidCron } from "@/features/services/lib/cron";

// The one acceptance table for a cron schedule. bex-api's validCronSchedule is
// tested against the same file (lego/backend/internal/apps/
// cron_schedule_vectors_test.go), and its answers are the source of truth: the
// form must accept exactly what the server accepts (w1/m145).
const VECTORS: { schedule: string; valid: boolean }[] = JSON.parse(
  readFileSync(
    `${process.cwd()}/../lego/backend/internal/apps/testdata/cron-schedule-vectors.json`,
    "utf8",
  ),
);

describe("isValidCron", () => {
  it.each(VECTORS.map((v) => [v.schedule, v.valid] as const))(
    "isValidCron(%j) is %s, the same as bex-api",
    (schedule, valid) => {
      expect(isValidCron(schedule)).toBe(valid);
    },
  );

  it("is checked against a non-trivial table", () => {
    expect(VECTORS.filter((v) => v.valid).length).toBeGreaterThanOrEqual(10);
    expect(VECTORS.filter((v) => !v.valid).length).toBeGreaterThanOrEqual(10);
  });
});

describe("describeCron", () => {
  it("names the common shapes", () => {
    expect(describeCron("* * * * *")).toBe("Every minute");
    expect(describeCron("*/5 * * * *")).toBe("Every 5 minutes");
    expect(describeCron("0 * * * *")).toBe("Every hour");
    expect(describeCron("30 * * * *")).toBe("Every hour at minute 30");
    expect(describeCron("0 */2 * * *")).toBe("Every 2 hours");
    expect(describeCron("0 0 * * *")).toBe("Every day at 00:00");
    expect(describeCron("30 9 * * *")).toBe("Every day at 09:30");
    expect(describeCron("0 8 * * 1")).toBe("Every Monday at 08:00");
    expect(describeCron("0 0 1 * *")).toBe("On day 1 of every month at 00:00");
  });

  it("returns null for invalid input and unhandled shapes", () => {
    expect(describeCron("99 99 * * *")).toBeNull();
    expect(describeCron("not a cron")).toBeNull();
    expect(describeCron("0 0 * * 1-5")).toBeNull();
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
      expect(describeCron(named)).toBe(describeCron(numeric));
    });

    it("produces the numeric path's exact phrases for named single tokens", () => {
      expect(describeCron("0 0 * * MON")).toBe("Every Monday at 00:00");
      expect(describeCron("0 0 * * SUN")).toBe("Every Sunday at 00:00");
    });
  });

  // w1/m145: 7 is not Sunday to bex-api, so it gets no Sunday preview; ? is
  // robfig's synonym for *, so it previews exactly like *.
  it("previews ? like * and gives 7 no preview", () => {
    expect(describeCron("0 0 * * 7")).toBeNull();
    expect(describeCron("0 0 ? * *")).toBe("Every day at 00:00");
    expect(describeCron("0 8 ? * 1")).toBe("Every Monday at 08:00");
    expect(describeCron("? ? ? ? ?")).toBe("Every minute");
  });
});

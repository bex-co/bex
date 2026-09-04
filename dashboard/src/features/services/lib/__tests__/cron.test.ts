import { describe, expect, it } from "vitest";

import { describeCron, isValidCron } from "@/features/services/lib/cron";

describe("isValidCron", () => {
  it("accepts ordinary valid 5-field expressions", () => {
    for (const ok of [
      "*/5 * * * *",
      "0 0 * * *",
      "0 8 * * 1",
      "15 3 1-5 * *",
      "0 0 1 1 *",
      "0 0 * * MON",
      "0 0 * JAN *",
      "0,30 * * * *",
      "0 9-17 * * 1-5",
      "0 0 * * 7",
    ]) {
      expect(isValidCron(ok), ok).toBe(true);
    }
  });

  it("rejects the wrong number of fields", () => {
    for (const bad of ["", "* * *", "not a cron", "* * * * * *", "*"]) {
      expect(isValidCron(bad), bad).toBe(false);
    }
  });

  it("rejects 5-field expressions with out-of-range values (the 99 99 * * * bug)", () => {
    for (const bad of [
      "99 99 * * *",
      "0 24 * * *",
      "60 * * * *",
      "* * 0 * *",
      "* * 32 * *",
      "* * * 13 *",
      "* * * * 8",
      "*/0 * * * *",
      "5-2 * * * *",
      "abc * * * *",
    ]) {
      expect(isValidCron(bad), bad).toBe(false);
    }
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
      // 0 and 7 are both Sunday; the name resolves to 0.
      expect(describeCron("0 0 * * SUN")).toBe(describeCron("0 0 * * 7"));
    });
  });
});

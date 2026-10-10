import { describe, it, expect } from "vitest";
import {
  formatRelativeAge,
  formatRelativeUntil,
  commandPromptPrefix,
} from "@/features/services/lib/format";
import { translator } from "@/test/translator";

const t = translator("en");

describe("commandPromptPrefix", () => {
  it("renders '<rootDir>/ $' when a root dir is set", () => {
    expect(commandPromptPrefix("backend")).toBe("backend/ $");
    // Trailing slashes are normalized (shared with rootDirPrefix).
    expect(commandPromptPrefix("apps/web/")).toBe("apps/web/ $");
  });

  it("falls back to a bare '$' prompt when no root dir is set", () => {
    expect(commandPromptPrefix(null)).toBe("$");
    expect(commandPromptPrefix(undefined)).toBe("$");
    expect(commandPromptPrefix("")).toBe("$");
    expect(commandPromptPrefix("   ")).toBe("$");
  });
});

describe("formatRelativeAge", () => {
  const now = Date.parse("2026-07-06T00:00:00Z");

  it("renders Render-style compact units", () => {
    const ago = (ms: number) => new Date(now - ms).toISOString();
    const SEC = 1000,
      MIN = 60 * SEC,
      HOUR = 60 * MIN,
      DAY = 24 * HOUR;
    expect(formatRelativeAge(ago(30 * SEC), t, now)).toBe("now");
    expect(formatRelativeAge(ago(5 * MIN), t, now)).toBe("5m");
    expect(formatRelativeAge(ago(3 * HOUR), t, now)).toBe("3h");
    expect(formatRelativeAge(ago(5 * DAY), t, now)).toBe("5d");
    expect(formatRelativeAge(ago(60 * DAY), t, now)).toBe("2mo");
    expect(formatRelativeAge(ago(400 * DAY), t, now)).toBe("1y");
  });

  it("returns an em dash for missing or unparseable input", () => {
    expect(formatRelativeAge(null, t, now)).toBe("—");
    expect(formatRelativeAge(undefined, t, now)).toBe("—");
    expect(formatRelativeAge("not-a-date", t, now)).toBe("—");
  });

  it("never returns a negative age for a future timestamp", () => {
    const future = new Date(now + 60_000).toISOString();
    expect(formatRelativeAge(future, t, now)).toBe("now");
  });
});

describe("formatRelativeUntil", () => {
  const now = Date.parse("2026-07-06T00:00:00Z");
  const until = (ms: number) => new Date(now + ms).toISOString();
  const SEC = 1000,
    MIN = 60 * SEC,
    HOUR = 60 * MIN,
    DAY = 24 * HOUR;

  it("renders compact 'in N' units for a future cron run", () => {
    expect(formatRelativeUntil(until(5 * MIN), t, now)).toBe("in 5m");
    expect(formatRelativeUntil(until(3 * HOUR), t, now)).toBe("in 3h");
    expect(formatRelativeUntil(until(2 * DAY), t, now)).toBe("in 2d");
  });

  it("collapses an imminent or past run to 'now'", () => {
    expect(formatRelativeUntil(until(30 * SEC), t, now)).toBe("now");
    expect(
      formatRelativeUntil(new Date(now - 60_000).toISOString(), t, now),
    ).toBe("now");
  });

  it("returns an em dash for missing or unparseable input", () => {
    expect(formatRelativeUntil(null, t, now)).toBe("—");
    expect(formatRelativeUntil(undefined, t, now)).toBe("—");
    expect(formatRelativeUntil("not-a-date", t, now)).toBe("—");
  });
});

// w4/m188: both formatters took no language and returned English literals, so
// a Chinese page read "上次运行：now" / "下次运行：in 83d". They now format through
// the caller's translator; English output above is unchanged.
describe("relative time in Chinese", () => {
  const zh = translator("zh");
  const now = Date.parse("2026-10-09T11:34:00Z");
  const SEC = 1000,
    MIN = 60 * SEC,
    HOUR = 60 * MIN,
    DAY = 24 * HOUR;
  const at = (ms: number) => new Date(now + ms).toISOString();

  it("localizes every past-age bucket at its boundaries", () => {
    expect(formatRelativeAge(at(-14 * SEC), zh, now)).toBe("刚刚");
    expect(formatRelativeAge(at(-59 * SEC), zh, now)).toBe("刚刚");
    expect(formatRelativeAge(at(-60 * SEC), zh, now)).toBe("1分钟");
    expect(formatRelativeAge(at(-2 * MIN), zh, now)).toBe("2分钟");
    expect(formatRelativeAge(at(-59 * MIN), zh, now)).toBe("59分钟");
    expect(formatRelativeAge(at(-HOUR), zh, now)).toBe("1小时");
    expect(formatRelativeAge(at(-23 * HOUR), zh, now)).toBe("23小时");
    expect(formatRelativeAge(at(-DAY), zh, now)).toBe("1天");
    expect(formatRelativeAge(at(-29 * DAY), zh, now)).toBe("29天");
    expect(formatRelativeAge(at(-30 * DAY), zh, now)).toBe("1个月");
    expect(formatRelativeAge(at(-359 * DAY), zh, now)).toBe("11个月");
    expect(formatRelativeAge(at(-365 * DAY), zh, now)).toBe("1年");
    expect(formatRelativeAge(at(-400 * DAY), zh, now)).toBe("1年");
    // A future instant still clamps to the "just now" bucket.
    expect(formatRelativeAge(at(MIN), zh, now)).toBe("刚刚");
  });

  it("localizes every countdown bucket, including the recorded 83 days", () => {
    expect(formatRelativeUntil(at(30 * SEC), zh, now)).toBe("现在");
    expect(formatRelativeUntil(at(-MIN), zh, now)).toBe("现在");
    expect(formatRelativeUntil(at(5 * MIN), zh, now)).toBe("5分钟后");
    expect(formatRelativeUntil(at(3 * HOUR), zh, now)).toBe("3小时后");
    expect(formatRelativeUntil(at(2 * DAY), zh, now)).toBe("2天后");
    // The finding's annual cron: next run 2027-01-01T00:00:00Z.
    expect(formatRelativeUntil("2027-01-01T00:00:00Z", zh, now)).toBe("83天后");
    expect(formatRelativeUntil("2027-01-01T00:00:00Z", t, now)).toBe("in 83d");
  });

  it("keeps the language-neutral em dash for missing or invalid input", () => {
    expect(formatRelativeAge(null, zh, now)).toBe("—");
    expect(formatRelativeAge("not-a-date", zh, now)).toBe("—");
    expect(formatRelativeUntil(undefined, zh, now)).toBe("—");
  });
});

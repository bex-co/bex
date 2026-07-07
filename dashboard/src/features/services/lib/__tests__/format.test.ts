import { describe, it, expect } from "vitest";
import { formatRelativeAge } from "@/features/services/lib/format";

describe("formatRelativeAge", () => {
  const now = Date.parse("2026-07-06T00:00:00Z");

  it("renders Render-style compact units", () => {
    const ago = (ms: number) => new Date(now - ms).toISOString();
    const SEC = 1000,
      MIN = 60 * SEC,
      HOUR = 60 * MIN,
      DAY = 24 * HOUR;
    expect(formatRelativeAge(ago(30 * SEC), now)).toBe("now");
    expect(formatRelativeAge(ago(5 * MIN), now)).toBe("5m");
    expect(formatRelativeAge(ago(3 * HOUR), now)).toBe("3h");
    expect(formatRelativeAge(ago(5 * DAY), now)).toBe("5d");
    expect(formatRelativeAge(ago(60 * DAY), now)).toBe("2mo");
    expect(formatRelativeAge(ago(400 * DAY), now)).toBe("1y");
  });

  it("returns an em dash for missing or unparseable input", () => {
    expect(formatRelativeAge(null, now)).toBe("—");
    expect(formatRelativeAge(undefined, now)).toBe("—");
    expect(formatRelativeAge("not-a-date", now)).toBe("—");
  });

  it("never returns a negative age for a future timestamp", () => {
    const future = new Date(now + 60_000).toISOString();
    expect(formatRelativeAge(future, now)).toBe("now");
  });
});

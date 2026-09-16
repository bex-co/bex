import { describe, expect, it } from "vitest";

import { formatInstantDetails, formatTimeAgo } from "../";

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

  it("returns null for a missing or unparseable instant", () => {
    expect(formatTimeAgo(null, opts)).toBeNull();
    expect(formatTimeAgo("", opts)).toBeNull();
    expect(formatTimeAgo("not-a-date", opts)).toBeNull();
  });
});

describe("formatInstantDetails", () => {
  it("renders the same instant as UTC and as Unix seconds", () => {
    const details = formatInstantDetails(INSTANT)!;
    expect(details.utc).toBe("July 16, 2026 at 12:01:30 AM UTC");
    expect(details.unix).toBe("1784160090");
  });

  it("renders the local reading in the same calendar style, with seconds and a zone", () => {
    const details = formatInstantDetails(INSTANT)!;
    // The runner's timezone decides the clock reading and zone name; the
    // shape ("<Month> <d>, <yyyy> at <h:mm:ss> <AM|PM> <zone>") does not.
    expect(details.local).toMatch(
      /^[A-Z][a-z]+ \d{1,2}, \d{4} at \d{1,2}:\d{2}:\d{2} [AP]M \S+/,
    );
    expect(
      new Intl.DateTimeFormat("en-US", {
        dateStyle: "long",
        timeStyle: "long",
      }).format(new Date(INSTANT)),
    ).toBe(details.local);
  });

  it("returns null for a missing or unparseable instant", () => {
    expect(formatInstantDetails(null)).toBeNull();
    expect(formatInstantDetails(undefined)).toBeNull();
    expect(formatInstantDetails("garbage")).toBeNull();
  });
});

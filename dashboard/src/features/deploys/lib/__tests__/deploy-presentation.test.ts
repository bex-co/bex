import { describe, expect, it } from "vitest";
import {
  deployMatchesSearch,
  formatDeployDuration,
  formatDeployTimestamp,
} from "../deploy-presentation";

describe("deploy presentation", () => {
  it.each([
    ["2026-07-16T00:00:00Z", "2026-07-16T00:00:09Z", "9s"],
    ["2026-07-16T00:00:00Z", "2026-07-16T00:01:30Z", "1m 30s"],
    ["2026-07-16T00:00:00Z", "2026-07-16T02:05:00Z", "2h 5m"],
  ])("formats %s..%s as %s", (start, finish, expected) => {
    expect(formatDeployDuration(start, finish)).toBe(expected);
  });

  it("omits missing, malformed, and backwards intervals", () => {
    expect(formatDeployDuration(null, "2026-07-16T00:00:00Z")).toBeNull();
    expect(formatDeployDuration("bad", "2026-07-16T00:00:00Z")).toBeNull();
    expect(
      formatDeployDuration("2026-07-16T00:01:00Z", "2026-07-16T00:00:00Z"),
    ).toBeNull();
    expect(formatDeployTimestamp("bad")).toBeNull();
  });

  it("formats a timestamp in the dashboard's standard style", () => {
    // Zone-less input parses as local time, so this holds in any runner TZ.
    expect(formatDeployTimestamp("2026-07-16T00:57:00")).toBe(
      "July 16, 2026 at 12:57 AM",
    );
  });

  it("does not describe a positive sub-second deploy as zero seconds", () => {
    expect(
      formatDeployDuration(
        "2026-07-16T00:00:00.100Z",
        "2026-07-16T00:00:00.900Z",
      ),
    ).toBe("1s");
  });

  it("searches id, full commit SHA, and message case-insensitively", () => {
    const deploy = {
      id: "dep-D9CP31A",
      commitId: "ABC1234DEF5678",
      commitMessage: "Fix: Broken startup",
    };

    expect(deployMatchesSearch(deploy, "d9cp")).toBe(true);
    expect(deployMatchesSearch(deploy, "def5678")).toBe(true);
    expect(deployMatchesSearch(deploy, "broken STARTUP")).toBe(true);
    expect(deployMatchesSearch(deploy, "unrelated")).toBe(false);
  });
});

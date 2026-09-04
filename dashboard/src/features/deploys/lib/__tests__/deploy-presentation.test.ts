import { describe, expect, it } from "vitest";
import {
  deployMatchesSearch,
  deployRowTimestamp,
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

  it("labels each terminal state with its own verb and finish time (w6/051)", () => {
    const createdAt = "2026-08-08T19:00:00Z";
    const finishedAt = "2026-08-08T19:24:00Z";

    // A shipped deploy is stamped when it went live, not when its row opened.
    expect(
      deployRowTimestamp({ status: "live", createdAt, finishedAt }),
    ).toEqual({ key: "deploys.deployedAt", iso: finishedAt });
    // A deactivated deploy was live once — it keeps the "Deployed" verb.
    expect(
      deployRowTimestamp({ status: "deactivated", createdAt, finishedAt }),
    ).toEqual({ key: "deploys.deployedAt", iso: finishedAt });
    expect(
      deployRowTimestamp({ status: "canceled", createdAt, finishedAt }),
    ).toEqual({ key: "deploys.canceledAt", iso: finishedAt });
    for (const status of [
      "build_failed",
      "pre_deploy_failed",
      "update_failed",
    ]) {
      expect(deployRowTimestamp({ status, createdAt, finishedAt })).toEqual({
        key: "deploys.failedAt",
        iso: finishedAt,
      });
    }
  });

  it("falls back to createdAt when a finished deploy stored no finish time", () => {
    const createdAt = "2026-08-08T19:00:00Z";
    expect(
      deployRowTimestamp({ status: "live", createdAt, finishedAt: null }),
    ).toEqual({ key: "deploys.deployedAt", iso: createdAt });
    expect(
      deployRowTimestamp({ status: "canceled", createdAt, finishedAt: null }),
    ).toEqual({ key: "deploys.canceledAt", iso: createdAt });
  });

  it("stamps unfinished (and unknown-status) deploys with their creation time", () => {
    const createdAt = "2026-08-08T19:00:00Z";
    for (const status of [
      "created",
      "queued",
      "build_in_progress",
      "pre_deploy_in_progress",
      "update_in_progress",
      "something_new",
    ]) {
      expect(
        deployRowTimestamp({ status, createdAt, finishedAt: null }),
      ).toEqual({ key: "deploys.createdAt", iso: createdAt });
    }
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

import { describe, expect, it } from "vitest";
import {
  cronRunElapsedMs,
  cronRunIconKind,
  cronRunLogsSearch,
  cronRunMatchesSearch,
  cronRunStatus,
  latestSuccessfulRunId,
} from "@/features/services/lib/cron-run-list";
import type { CronRunView } from "@/features/services/types";

const run = (patch: Partial<CronRunView>): CronRunView => ({
  id: "crr-d9aaaaaaaaaaaaaaaaaa",
  startedAt: null,
  finishedAt: null,
  status: "pending",
  ...patch,
});

describe("cronRunStatus", () => {
  it.each([
    ["successful", "successful"],
    ["succeeded", "successful"],
    ["unsuccessful", "unsuccessful"],
    ["failed", "unsuccessful"],
    ["canceled", "canceled"],
    ["pending", "pending"],
    ["running", "pending"],
    ["Successful", "successful"],
  ])("reads %s as Render's %s", (status, want) => {
    expect(cronRunStatus({ status })).toBe(want);
  });
});

describe("latestSuccessfulRunId and the status icon", () => {
  it("marks the newest successful run, not an older one or a failure", () => {
    const runs = [
      run({ id: "crr-3", status: "unsuccessful" }),
      run({ id: "crr-2", status: "successful" }),
      run({ id: "crr-1", status: "successful" }),
    ];
    expect(latestSuccessfulRunId(runs)).toBe("crr-2");
    expect(latestSuccessfulRunId([runs[0]])).toBeNull();
  });

  it("draws Render's icon per status: filled for the newest success, a spinner while in progress", () => {
    expect(cronRunIconKind("successful", true)).toBe("live");
    expect(cronRunIconKind("successful", false)).toBe("succeeded");
    expect(cronRunIconKind("unsuccessful", false)).toBe("failed");
    expect(cronRunIconKind("canceled", false)).toBe("canceled");
    expect(cronRunIconKind("pending", false)).toBe("in_progress");
  });
});

describe("cronRunLogsSearch", () => {
  const now = Date.parse("2026-11-22T12:00:00Z");

  it("opens the Logs from a minute before the run to a minute after it", () => {
    expect(
      cronRunLogsSearch(
        run({
          startedAt: "2026-11-21T09:00:00Z",
          finishedAt: "2026-11-21T09:00:14Z",
        }),
        now,
      ),
    ).toEqual({
      range: "custom",
      rangeStart: "2026-11-21T08:59:00.000Z",
      rangeEnd: "2026-11-21T09:01:14.000Z",
    });
  });

  it("ends a minute after now while the run is in progress", () => {
    expect(
      cronRunLogsSearch(run({ startedAt: "2026-11-22T11:58:30Z" }), now),
    ).toEqual({
      range: "custom",
      rangeStart: "2026-11-22T11:57:30.000Z",
      rangeEnd: "2026-11-22T12:01:00.000Z",
    });
  });

  it("covers the hour before the end of a run that never started", () => {
    expect(
      cronRunLogsSearch(
        run({ status: "canceled", finishedAt: "2026-11-22T10:00:00Z" }),
        now,
      ),
    ).toEqual({
      range: "custom",
      rangeStart: "2026-11-22T09:00:00.000Z",
      rangeEnd: "2026-11-22T10:01:00.000Z",
    });
  });
});

describe("cronRunElapsedMs", () => {
  it("is the span of a finished run", () => {
    expect(
      cronRunElapsedMs(
        run({
          startedAt: "2026-11-21T09:00:00Z",
          finishedAt: "2026-11-21T09:00:14Z",
        }),
        0,
      ),
    ).toBe(14_000);
  });

  it("counts a run in progress up to now in whole seconds", () => {
    const started = Date.parse("2026-11-22T12:00:00Z");
    expect(
      cronRunElapsedMs(
        run({ startedAt: "2026-11-22T12:00:00Z" }),
        started + 3_900,
      ),
    ).toBe(3_000);
  });

  it("is null before the run starts", () => {
    expect(cronRunElapsedMs(run({}), Date.now())).toBeNull();
  });
});

describe("cronRunMatchesSearch", () => {
  const fields = ["Run failed", "failed", null, "Manual", "ops@example.com"];

  it("keeps every run for an empty or blank query", () => {
    expect(cronRunMatchesSearch(run({}), fields, "")).toBe(true);
    expect(cronRunMatchesSearch(run({}), fields, "  / ")).toBe(true);
  });

  it("matches each word in any field, ignoring case", () => {
    expect(cronRunMatchesSearch(run({}), fields, "FAILED")).toBe(true);
    expect(cronRunMatchesSearch(run({}), fields, "manual ops@")).toBe(true);
    expect(cronRunMatchesSearch(run({}), fields, "manual/failed")).toBe(true);
    expect(cronRunMatchesSearch(run({}), fields, "manual scheduled")).toBe(
      false,
    );
  });

  it("matches a whole run id, and never a fragment of one", () => {
    const target = run({ id: "crr-d9aaaaaaaaaaaaaaaaaa" });
    expect(
      cronRunMatchesSearch(target, fields, " crr-d9aaaaaaaaaaaaaaaaaa "),
    ).toBe(true);
    expect(
      cronRunMatchesSearch(target, fields, "crr-d9bbbbbbbbbbbbbbbbbb"),
    ).toBe(false);
    expect(cronRunMatchesSearch(target, fields, "crr-d9a")).toBe(false);
  });
});

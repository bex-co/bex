import {
  cronRunDuration,
  currentCronRun,
  isActiveCronRun,
  isTerminalCronRun,
  mergeCronRuns,
  composeCronRunPages,
  type CronRunSummary,
} from "../cron-history";

const run = (id: string, status: string): CronRunSummary => ({
  id,
  status,
  startedAt: null,
  finishedAt: null,
});

describe("cron history", () => {
  it("recognizes only pending/running as active and terminal statuses as terminal", () => {
    expect(isActiveCronRun("pending")).toBe(true);
    expect(isActiveCronRun("RUNNING")).toBe(true);
    expect(isActiveCronRun("successful")).toBe(false);
    expect(isTerminalCronRun("canceled")).toBe(true);
    expect(isTerminalCronRun("failed")).toBe(true);
    expect(isTerminalCronRun("pending")).toBe(false);
  });

  it("selects only the current active run and keeps cursor pages stable", () => {
    const first = [run("crr-new", "successful"), run("crr-live", "running")];
    const page = [run("crr-live", "running"), run("crr-old", "failed")];
    expect(currentCronRun(first)?.id).toBe("crr-live");
    expect(mergeCronRuns(first, page).map((item) => item.id)).toEqual([
      "crr-new",
      "crr-live",
      "crr-old",
    ]);
  });

  it("represents empty history without inventing a current run", () => {
    expect(currentCronRun([])).toBe(null);
    expect(mergeCronRuns([], [])).toEqual([]);
  });

  it("replaces a duplicate with refreshed truth without duplicating its id", () => {
    expect(
      mergeCronRuns(
        [run("crr-live", "running")],
        [run("crr-live", "canceled")],
      ),
    ).toEqual([run("crr-live", "canceled")]);
    expect(
      composeCronRunPages(
        [run("crr-new", "pending"), run("crr-live", "canceled")],
        [run("crr-live", "running"), run("crr-old", "successful")],
      ),
    ).toEqual([
      run("crr-new", "pending"),
      run("crr-live", "canceled"),
      run("crr-old", "successful"),
    ]);
  });

  it("formats only valid completed durations", () => {
    expect(
      cronRunDuration({
        id: "crr-one",
        status: "successful",
        startedAt: "2026-08-02T00:00:00Z",
        finishedAt: "2026-08-02T00:01:05Z",
      }),
    ).toBe("1m 5s");
    expect(cronRunDuration(run("crr-live", "running"))).toBe(null);
  });
});

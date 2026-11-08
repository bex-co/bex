import { describe, expect, it } from "vitest";
import { translator } from "@/test/translator";
import {
  browserTimeZone,
  defaultScheduleDraft,
  describeSchedule,
  describeDraft,
  draftToScheduleInput,
  scheduleProblems,
  scheduleToDraft,
  type ScheduleDraft,
  type ScheduleView,
} from "@/features/agent-automations/lib/schedule";

const NOW = Date.parse("2026-11-07T12:00:00Z");

function draft(over: Partial<ScheduleDraft>): ScheduleDraft {
  return { ...defaultScheduleDraft("Europe/Berlin"), ...over };
}

function view(over: Partial<ScheduleView>): ScheduleView {
  return {
    frequency: "day",
    minute: null,
    at: null,
    weekdays: null,
    runAt: null,
    cron: null,
    timezone: "Europe/Berlin",
    endsAt: null,
    ...over,
  };
}

describe("schedule draft -> API input (ADR101 D2)", () => {
  it("defaults the zone to the browser's own and starts on a daily 09:00", () => {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(browserTimeZone()).toBe(zone);
    expect(defaultScheduleDraft().timezone).toBe(zone);
    expect(defaultScheduleDraft().frequency).toBe("day");
  });

  it("hourly sends only the minute", () => {
    expect(
      draftToScheduleInput(draft({ frequency: "hour", minute: " 5 " })),
    ).toEqual({ frequency: "hour", minute: 5, timezone: "Europe/Berlin" });
  });

  it("daily sends only the time", () => {
    expect(
      draftToScheduleInput(draft({ frequency: "day", at: "07:30" })),
    ).toEqual({ frequency: "day", at: "07:30", timezone: "Europe/Berlin" });
  });

  it("weekly sends the time and the days in week order", () => {
    expect(
      draftToScheduleInput(
        draft({ frequency: "week", at: "18:00", weekdays: ["fri", "mon"] }),
      ),
    ).toEqual({
      frequency: "week",
      at: "18:00",
      weekdays: ["mon", "fri"],
      timezone: "Europe/Berlin",
    });
  });

  it("once reads the wall time in the chosen zone, not the browser's", () => {
    // 15:00 in Berlin on 12 Nov 2026 is CET (UTC+1).
    expect(
      draftToScheduleInput(
        draft({ frequency: "once", runAt: "2026-11-12T15:00" }),
      ),
    ).toEqual({
      frequency: "once",
      runAt: "2026-11-12T14:00:00.000Z",
      timezone: "Europe/Berlin",
    });
    // Summer time: the same wall clock is UTC+2.
    expect(
      draftToScheduleInput(
        draft({ frequency: "once", runAt: "2026-07-12T15:00" }),
      ).runAt,
    ).toBe("2026-07-12T13:00:00.000Z");
  });

  it("custom sends the trimmed cron with single spaces", () => {
    expect(
      draftToScheduleInput(
        draft({ frequency: "custom", cron: "  0  9 * * 1-5 " }),
      ),
    ).toEqual({
      frequency: "custom",
      cron: "0 9 * * 1-5",
      timezone: "Europe/Berlin",
    });
  });

  it("ends-on becomes the exclusive start of the next day in the zone", () => {
    expect(
      draftToScheduleInput(draft({ frequency: "day", endsOn: "2026-12-31" }))
        .endsAt,
    ).toBe("2026-12-31T23:00:00.000Z");
  });

  it("a once schedule never carries an end", () => {
    expect(
      draftToScheduleInput(
        draft({
          frequency: "once",
          runAt: "2026-11-12T15:00",
          endsOn: "2027-01-01",
        }),
      ).endsAt,
    ).toBeUndefined();
  });
});

describe("scheduleProblems", () => {
  it("accepts each valid frequency", () => {
    expect(scheduleProblems(draft({ frequency: "day" }), NOW)).toEqual({});
    expect(
      scheduleProblems(draft({ frequency: "hour", minute: "0" }), NOW),
    ).toEqual({});
    expect(
      scheduleProblems(
        draft({ frequency: "custom", cron: "*/10 * * * *" }),
        NOW,
      ),
    ).toEqual({});
    expect(
      scheduleProblems(
        draft({ frequency: "once", runAt: "2026-12-01T09:00" }),
        NOW,
      ),
    ).toEqual({});
  });

  it("flags each frequency's own missing input", () => {
    expect(
      scheduleProblems(draft({ frequency: "hour", minute: "60" }), NOW),
    ).toEqual({
      minute: "agentAutomations.problem.minute",
    });
    expect(scheduleProblems(draft({ frequency: "day", at: "" }), NOW)).toEqual({
      at: "agentAutomations.problem.at",
    });
    expect(
      scheduleProblems(draft({ frequency: "week", weekdays: [] }), NOW),
    ).toEqual({ weekdays: "agentAutomations.problem.weekdays" });
    expect(
      scheduleProblems(draft({ frequency: "once", runAt: "" }), NOW),
    ).toEqual({
      runAt: "agentAutomations.problem.runAtRequired",
    });
    expect(
      scheduleProblems(
        draft({ frequency: "once", runAt: "2026-11-01T09:00" }),
        NOW,
      ),
    ).toEqual({ runAt: "agentAutomations.problem.runAtPast" });
    expect(
      scheduleProblems(draft({ frequency: "custom", cron: "0 9 * *" }), NOW),
    ).toEqual({ cron: "agentAutomations.problem.cron" });
  });

  it("flags an unknown zone and a past end date", () => {
    expect(scheduleProblems(draft({ timezone: "Mars/Olympus" }), NOW)).toEqual({
      timezone: "agentAutomations.problem.timezone",
    });
    expect(scheduleProblems(draft({ endsOn: "2026-01-01" }), NOW)).toEqual({
      endsOn: "agentAutomations.problem.endsOnPast",
    });
  });
});

describe("scheduleToDraft", () => {
  it("round-trips a stored once schedule and an end date through the zone", () => {
    const once = scheduleToDraft(
      view({
        frequency: "once",
        runAt: "2026-11-12T14:00:00Z",
      }),
    );
    expect(once.runAt).toBe("2026-11-12T15:00");
    const ends = scheduleToDraft(
      view({ frequency: "day", at: "09:00", endsAt: "2026-12-31T23:00:00Z" }),
    );
    expect(ends.endsOn).toBe("2026-12-31");
    expect(draftToScheduleInput(ends).endsAt).toBe("2026-12-31T23:00:00.000Z");
  });
});

describe("describeSchedule (the trigger in words)", () => {
  const en = translator("en");

  it("reads each frequency", () => {
    expect(
      describeSchedule(view({ frequency: "day", at: "09:00" }), en, "en"),
    ).toBe("Every day at 09:00 · Europe/Berlin");
    expect(
      describeSchedule(view({ frequency: "hour", minute: 5 }), en, "en"),
    ).toBe("Every hour at :05 · Europe/Berlin");
    expect(
      describeSchedule(
        view({ frequency: "week", at: "08:15", weekdays: ["fri", "mon"] }),
        en,
        "en",
      ),
    ).toBe("Every Mon, Fri at 08:15 · Europe/Berlin");
    expect(
      describeSchedule(
        view({ frequency: "custom", cron: "0 9 * * 1-5" }),
        en,
        "en",
      ),
    ).toBe("Cron 0 9 * * 1-5 · Europe/Berlin");
  });

  it("shows a one-off in the schedule's own zone, not the viewer's", () => {
    const text = describeSchedule(
      view({ frequency: "once", runAt: "2026-11-12T14:00:00Z" }),
      en,
      "en",
    );
    expect(text).toMatch(/^Once on Nov 12, 2026/);
    expect(text).toContain("3:00");
    expect(text.endsWith("· Europe/Berlin")).toBe(true);
  });

  it("appends the last day when the schedule ends", () => {
    expect(
      describeSchedule(
        view({ frequency: "day", at: "09:00", endsAt: "2026-12-31T23:00:00Z" }),
        en,
        "en",
      ),
    ).toBe("Every day at 09:00 · Europe/Berlin · until Dec 31, 2026");
  });

  it("is translated", () => {
    expect(
      describeSchedule(
        view({ frequency: "day", at: "09:00" }),
        translator("zh"),
        "zh",
      ),
    ).toBe("每天 09:00 · Europe/Berlin");
  });

  it("summarizes a valid draft and nothing for an invalid one", () => {
    expect(
      describeDraft(draft({ frequency: "day", at: "09:00" }), en, "en"),
    ).toBe("Every day at 09:00 · Europe/Berlin");
    expect(
      describeDraft(draft({ frequency: "day", at: "" }), en, "en"),
    ).toBeNull();
  });
});

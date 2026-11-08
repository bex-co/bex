// The schedule trigger of an agent automation (ADR101 D2) as the form edits it:
// a string-valued draft, its conversion to and from the API's input/view, the
// client-side checks that mirror the server's shape rules, and the schedule in
// words. The server stays the authority (minimum cron interval, past instants,
// zone database), so only shape errors are caught here.

import type { AgentAutomationScheduleInput } from "@/graphql/definitions";

export const FREQUENCIES = ["hour", "day", "week", "once", "custom"] as const;
export type Frequency = (typeof FREQUENCIES)[number];

export const WEEKDAYS = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
] as const;
export type Weekday = (typeof WEEKDAYS)[number];

/** A schedule as the API returns it (the fields the dashboard reads). */
export interface ScheduleView {
  frequency: string;
  minute: number | null;
  at: string | null;
  weekdays: readonly string[] | null;
  runAt: string | null;
  cron: string | null;
  timezone: string;
  endsAt: string | null;
}

/** Everything the picker edits, as typed: strings, so a half-typed value survives. */
export interface ScheduleDraft {
  frequency: Frequency;
  /** Minute past the hour, for hourly. */
  minute: string;
  /** "HH:MM", 24-hour, for daily and weekly. */
  at: string;
  weekdays: Weekday[];
  /** "YYYY-MM-DDTHH:mm" wall time in `timezone`, for once. */
  runAt: string;
  cron: string;
  timezone: string;
  /** "YYYY-MM-DD" last day the schedule may fire (in `timezone`), or "". */
  endsOn: string;
}

export type ScheduleField =
  | "frequency"
  | "minute"
  | "at"
  | "weekdays"
  | "runAt"
  | "cron"
  | "timezone"
  | "endsOn";

/** The picker fields a frequency shows, so a refusal naming a hidden one can be shown schedule-wide. */
export function visibleScheduleFields(frequency: Frequency): ScheduleField[] {
  const own: Record<Frequency, ScheduleField[]> = {
    hour: ["minute", "endsOn"],
    day: ["at", "endsOn"],
    week: ["at", "weekdays", "endsOn"],
    once: ["runAt"],
    custom: ["cron", "endsOn"],
  };
  return ["frequency", "timezone", ...own[frequency]];
}

export type ScheduleProblems = Partial<Record<ScheduleField, string>>;

/** The browser's own zone, "UTC" when it cannot say. */
export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

export function defaultScheduleDraft(
  timezone: string = browserTimeZone(),
): ScheduleDraft {
  return {
    frequency: "day",
    minute: "0",
    at: "09:00",
    weekdays: ["mon", "tue", "wed", "thu", "fri"],
    runAt: "",
    cron: "",
    timezone,
    endsOn: "",
  };
}

const AT_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/;
const WALL_PATTERN = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/;
const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

function isKnownZone(zone: string): boolean {
  if (zone === "" || zone === "Local") return false;
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}

interface WallParts {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
}

/** The wall clock in `zone` at an instant. */
function wallPartsAt(ms: number, zone: string): WallParts {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: zone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).formatToParts(new Date(ms));
  const pick = (type: string) =>
    Number(parts.find((p) => p.type === type)?.value ?? "0");
  return {
    year: pick("year"),
    month: pick("month"),
    day: pick("day"),
    hour: pick("hour") % 24,
    minute: pick("minute"),
  };
}

function wallAsUtc(w: WallParts): number {
  return Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute);
}

/** The instant (epoch ms) a wall-clock reading denotes in `zone`. */
function zonedWallToInstant(w: WallParts, zone: string): number {
  const asUtc = wallAsUtc(w);
  // Two passes settle the offset across a DST change.
  let guess = asUtc - (wallAsUtc(wallPartsAt(asUtc, zone)) - asUtc);
  guess = asUtc - (wallAsUtc(wallPartsAt(guess, zone)) - guess);
  return guess;
}

const pad = (n: number, width = 2) => String(n).padStart(width, "0");

function wallToString(w: WallParts): string {
  return `${pad(w.year, 4)}-${pad(w.month)}-${pad(w.day)}T${pad(w.hour)}:${pad(w.minute)}`;
}

function parseWall(text: string): WallParts | null {
  const m = WALL_PATTERN.exec(text);
  if (!m) return null;
  const w = {
    year: Number(m[1]),
    month: Number(m[2]),
    day: Number(m[3]),
    hour: Number(m[4]),
    minute: Number(m[5]),
  };
  // Reject 2026-02-31 style dates the Date constructor would roll over.
  const d = new Date(wallAsUtc(w));
  return d.getUTCMonth() === w.month - 1 && d.getUTCDate() === w.day ? w : null;
}

function parseDay(text: string): WallParts | null {
  const m = DATE_PATTERN.exec(text);
  return m ? parseWall(`${text}T00:00`) : null;
}

/** RFC 3339 instant of a "once" wall time in `zone`, or null when malformed. */
export function runAtInstant(draft: ScheduleDraft): string | null {
  const wall = parseWall(draft.runAt);
  if (!wall || !isKnownZone(draft.timezone)) return null;
  return new Date(zonedWallToInstant(wall, draft.timezone)).toISOString();
}

/**
 * The exclusive end instant for an "ends on" day: the start of the NEXT day in
 * `zone`, so a fire on the chosen day itself still happens.
 */
export function endsOnInstant(draft: ScheduleDraft): string | null {
  const day = parseDay(draft.endsOn);
  if (!day || !isKnownZone(draft.timezone)) return null;
  const next = new Date(wallAsUtc(day) + 24 * 60 * 60 * 1000);
  const wall: WallParts = {
    year: next.getUTCFullYear(),
    month: next.getUTCMonth() + 1,
    day: next.getUTCDate(),
    hour: 0,
    minute: 0,
  };
  return new Date(zonedWallToInstant(wall, draft.timezone)).toISOString();
}

/**
 * Shape problems per field, as locale keys. `now` is injected so a past "once"
 * or end date is decided against one clock.
 */
export function scheduleProblems(
  draft: ScheduleDraft,
  now: number = Date.now(),
): ScheduleProblems {
  const out: ScheduleProblems = {};
  if (!isKnownZone(draft.timezone)) {
    out.timezone = "agentAutomations.problem.timezone";
  }
  switch (draft.frequency) {
    case "hour": {
      const minute = draft.minute.trim();
      if (!/^\d{1,2}$/.test(minute) || Number(minute) > 59) {
        out.minute = "agentAutomations.problem.minute";
      }
      break;
    }
    case "day":
      if (!AT_PATTERN.test(draft.at)) out.at = "agentAutomations.problem.at";
      break;
    case "week":
      if (!AT_PATTERN.test(draft.at)) out.at = "agentAutomations.problem.at";
      if (draft.weekdays.length === 0) {
        out.weekdays = "agentAutomations.problem.weekdays";
      }
      break;
    case "once": {
      const instant = runAtInstant(draft);
      if (draft.runAt === "" || (instant == null && !out.timezone)) {
        out.runAt = "agentAutomations.problem.runAtRequired";
      } else if (instant != null && Date.parse(instant) <= now) {
        out.runAt = "agentAutomations.problem.runAtPast";
      }
      break;
    }
    case "custom": {
      const fields = draft.cron.trim().split(/\s+/);
      if (draft.cron.trim() === "" || fields.length !== 5) {
        out.cron = "agentAutomations.problem.cron";
      }
      break;
    }
  }
  if (draft.endsOn !== "" && draft.frequency !== "once") {
    const end = endsOnInstant(draft);
    if (end == null) out.endsOn = "agentAutomations.problem.endsOn";
    else if (Date.parse(end) <= now) {
      out.endsOn = "agentAutomations.problem.endsOnPast";
    }
  }
  return out;
}

/** The API input for a draft that has no problems. */
export function draftToScheduleInput(
  draft: ScheduleDraft,
): AgentAutomationScheduleInput {
  const base: AgentAutomationScheduleInput = {
    frequency: draft.frequency,
    timezone: draft.timezone,
  };
  switch (draft.frequency) {
    case "hour":
      base.minute = Number(draft.minute.trim());
      break;
    case "day":
      base.at = draft.at;
      break;
    case "week":
      base.at = draft.at;
      base.weekdays = WEEKDAYS.filter((d) => draft.weekdays.includes(d));
      break;
    case "once":
      base.runAt = runAtInstant(draft);
      break;
    case "custom":
      base.cron = draft.cron.trim().replace(/\s+/g, " ");
      break;
  }
  if (draft.endsOn !== "" && draft.frequency !== "once") {
    base.endsAt = endsOnInstant(draft);
  }
  return base;
}

function isFrequency(value: string): value is Frequency {
  return (FREQUENCIES as readonly string[]).includes(value);
}

/** The draft an existing automation's schedule opens the edit form with. */
export function scheduleToDraft(schedule: ScheduleView): ScheduleDraft {
  const draft = defaultScheduleDraft(schedule.timezone);
  draft.frequency = isFrequency(schedule.frequency)
    ? schedule.frequency
    : "day";
  if (schedule.minute != null) draft.minute = String(schedule.minute);
  if (schedule.at) draft.at = schedule.at;
  if (schedule.weekdays?.length) {
    draft.weekdays = WEEKDAYS.filter((d) => schedule.weekdays?.includes(d));
  }
  if (schedule.runAt) {
    const ms = Date.parse(schedule.runAt);
    if (!Number.isNaN(ms) && isKnownZone(schedule.timezone)) {
      draft.runAt = wallToString(wallPartsAt(ms, schedule.timezone));
    }
  }
  if (schedule.cron) draft.cron = schedule.cron;
  if (schedule.endsAt) {
    const ms = Date.parse(schedule.endsAt);
    if (!Number.isNaN(ms) && isKnownZone(schedule.timezone)) {
      // endsAt is exclusive: the last day a fire can land is the minute before.
      draft.endsOn = wallToString(
        wallPartsAt(ms - 60_000, schedule.timezone),
      ).slice(0, 10);
    }
  }
  return draft;
}

/** A translator as `useTranslations()` returns it. */
type Translator = (
  key: string,
  params?: Record<string, string | number>,
) => string;

function formatInZone(
  iso: string,
  zone: string,
  language: string,
  options: Intl.DateTimeFormatOptions,
): string {
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return iso;
  try {
    return new Intl.DateTimeFormat(language, {
      timeZone: isKnownZone(zone) ? zone : undefined,
      ...options,
    }).format(new Date(ms));
  } catch {
    return iso;
  }
}

/**
 * The trigger in words: "Every day at 09:00 · Europe/Berlin", "Once on Nov 12,
 * 2026, 3:00 PM · Europe/Berlin", "Cron 0 9 * * 1-5 · Europe/Berlin", with the
 * end date appended when there is one.
 */
export function describeSchedule(
  schedule: ScheduleView,
  t: Translator,
  language: string,
): string {
  const zone = schedule.timezone;
  let what: string;
  switch (schedule.frequency) {
    case "hour":
      what = t("agentAutomations.schedule.hour", {
        minute: pad(schedule.minute ?? 0),
      });
      break;
    case "day":
      what = t("agentAutomations.schedule.day", { at: schedule.at ?? "" });
      break;
    case "week":
      what = t("agentAutomations.schedule.week", {
        days: WEEKDAYS.filter((d) => schedule.weekdays?.includes(d))
          .map((d) => t(`agentAutomations.weekday.${d}`))
          .join(", "),
        at: schedule.at ?? "",
      });
      break;
    case "once":
      what = t("agentAutomations.schedule.once", {
        when: schedule.runAt
          ? formatInZone(schedule.runAt, zone, language, {
              dateStyle: "medium",
              timeStyle: "short",
            })
          : "",
      });
      break;
    case "custom":
      what = t("agentAutomations.schedule.custom", {
        cron: schedule.cron ?? "",
      });
      break;
    default:
      what = schedule.frequency;
  }
  let text = `${what} · ${zone}`;
  if (schedule.endsAt && schedule.frequency !== "once") {
    const last = Date.parse(schedule.endsAt) - 60_000;
    text += ` · ${t("agentAutomations.schedule.until", {
      date: formatInZone(new Date(last).toISOString(), zone, language, {
        dateStyle: "medium",
      }),
    })}`;
  }
  return text;
}

/** The draft in words, for the live summary under the picker. Null until valid. */
export function describeDraft(
  draft: ScheduleDraft,
  t: Translator,
  language: string,
): string | null {
  if (Object.keys(scheduleProblems(draft)).length > 0) return null;
  const input = draftToScheduleInput(draft);
  return describeSchedule(
    {
      frequency: input.frequency,
      minute: input.minute ?? null,
      at: input.at ?? null,
      weekdays: input.weekdays ?? null,
      runAt: input.runAt ?? null,
      cron: input.cron ?? null,
      timezone: input.timezone,
      endsAt: input.endsAt ?? null,
    },
    t,
    language,
  );
}

/** Zone names for the picker: the runtime's list, plus UTC and the current value. */
export function timeZoneOptions(current: string): string[] {
  let zones: string[];
  try {
    zones = Intl.supportedValuesOf("timeZone");
  } catch {
    zones = [];
  }
  const set = new Set(zones);
  set.add("UTC");
  if (current) set.add(current);
  return [...set].sort((a, b) => a.localeCompare(b));
}

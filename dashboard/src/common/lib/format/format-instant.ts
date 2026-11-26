import { formatDistanceStrict } from "date-fns";
import { LANGUAGE_TAGS } from "@/i18n/config";
import { formatDateTime } from "./format-date-time";
import { formatDuration } from "./format-duration";
import { dashboardLanguage, dateFnsLocale, languageOptions } from "./language";

export interface TimeAgoOptions {
  /**
   * The reference clock in epoch ms. Injected rather than read here so the
   * output is deterministic under test and so one page-level ticker
   * (`useNow`) can drive every row instead of each row reading its own clock.
   */
  now: number;
  /** The active i18n language ("en", "zh", "de", …) — picks the unit words. */
  language?: string;
  /**
   * Text for an instant under a minute old ("just now"), supplied already
   * translated by the caller. Also absorbs clock skew: a server-stamped
   * instant a few seconds ahead of the viewer's clock must not read
   * "in 3 seconds".
   */
  justNow: string;
}

/**
 * Long-form elapsed time since a past instant, Render's deploy-row style:
 * "2 hours ago", "3 days ago", "1 month ago". The compact "2h"/"3d" spelling
 * lives in `formatRelativeAge` (services list column); this is the
 * sentence-friendly form for "Deployed {timestamp}". Null when `iso` is
 * missing or unparseable.
 */
export function formatTimeAgo(
  iso: string | null | undefined,
  { now, language, justNow }: TimeAgoOptions,
): string | null {
  const then = parseInstant(iso);
  if (then === null) return null;
  if (now - then < 60_000) return justNow;
  return formatDistanceStrict(then, now, {
    addSuffix: true,
    locale: dateFnsLocale(language),
  });
}

export interface InstantDetails {
  /**
   * The viewer's zone as Render's card names it, en-US's short zone name
   * ("PST", "GMT+8", "UTC"); null when the runtime names none, which the card
   * labels Local.
   */
  zone: string | null;
  /** The viewer's clock: "Nov 22 06:41:32 PM", "11月22日 下午06:41:32". */
  local: string;
  /** What the local row copies: "2026-11-22T10:41:32.123-08:00". */
  localCopy: string;
  /** The same reading in UTC; null when the viewer's zone is UTC, whose
   *  card Render gives no second UTC row. */
  utc: string | null;
  /** What the UTC row copies, the ISO string "2026-11-22T18:41:32.123Z". */
  utcCopy: string;
  /** Unix milliseconds, "1795376492123", shown and copied. */
  unix: string;
}

// Render's `MMM D hh:mm:ss A` in the dashboard language: the day ("Nov 22",
// "11月22日") and the log line's clock ("06:41:32 PM", "下午06:41:32").
const MONTH_DAY: Intl.DateTimeFormatOptions = {
  month: "short",
  day: "numeric",
};
const CLOCK: Intl.DateTimeFormatOptions = {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: true,
};
// The wall clock's fields in a zone, to spell the local copy and its offset.
const FIELDS: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
};

const zonedFormats = new Map<string, Intl.DateTimeFormat>();
function zonedFormat(
  tag: string,
  timeZone: string,
  options: Intl.DateTimeFormatOptions,
) {
  const key = `${tag}|${timeZone}|${JSON.stringify(options)}`;
  let format = zonedFormats.get(key);
  if (!format) {
    format = new Intl.DateTimeFormat(tag, { ...options, timeZone });
    zonedFormats.set(key, format);
  }
  return format;
}

function cardReading(date: Date, language: string | undefined, zone: string) {
  const lang = dashboardLanguage(language);
  const tag = LANGUAGE_TAGS[lang];
  const day = zonedFormat(tag, zone, languageOptions(lang, MONTH_DAY));
  const clock = zonedFormat(tag, zone, languageOptions(lang, CLOCK));
  return `${day.format(date)} ${clock.format(date)}`;
}

// The zone's short name at `date` as `tag` writes it ("PST", "MEZ", "GMT-8").
function zoneName(date: Date, zone: string, tag = "en-US"): string | null {
  const part = zonedFormat(tag, zone, { timeZoneName: "short" })
    .formatToParts(date)
    .find((p) => p.type === "timeZoneName");
  return part?.value || null;
}

const pad = (n: number, width = 2) => String(n).padStart(width, "0");

// dayjs's `YYYY-MM-DDTHH:mm:ss.SSSZ` in `zone`: the wall clock to the
// millisecond and its offset from UTC ("+08:00", "+00:00").
function zonedIso(ms: number, zone: string): string {
  const fields: Record<string, number> = {};
  for (const part of zonedFormat("en-US", zone, FIELDS).formatToParts(
    new Date(ms),
  )) {
    if (part.type !== "literal") fields[part.type] = Number(part.value);
  }
  const millis = ((ms % 1000) + 1000) % 1000;
  const wall = Date.UTC(
    fields.year,
    fields.month - 1,
    fields.day,
    fields.hour,
    fields.minute,
    fields.second,
  );
  const offset = Math.round((wall - (ms - millis)) / 60_000);
  const sign = offset < 0 ? "-" : "+";
  return (
    `${pad(fields.year, 4)}-${pad(fields.month)}-${pad(fields.day)}` +
    `T${pad(fields.hour)}:${pad(fields.minute)}:${pad(fields.second)}` +
    `.${pad(millis, 3)}${sign}${pad(Math.floor(Math.abs(offset) / 60))}:${pad(Math.abs(offset) % 60)}`
  );
}

/** The zone the runtime reads local times in (UTC in the SSR container). */
function runtimeZone(): string {
  return new Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

/**
 * The exact instant behind a time, as Render's timestamp card
 * (`TimestampTooltipContent`) gives it: the viewer's zone and clock, UTC
 * unless that is the viewer's zone, and Unix milliseconds, each with the
 * string its copy button writes. The body of `InstantTooltip`. The local
 * reading follows `zone`, by default the *runtime's* (UTC in the SSR
 * container), so it belongs only in client-side output that is never part of
 * the hydrated markup, such as a tooltip that mounts on hover. Null when
 * `iso` is missing or unparseable.
 */
export function formatInstantDetails(
  iso: string | null | undefined,
  language?: string,
  zone: string = runtimeZone(),
): InstantDetails | null {
  const ms = parseInstant(iso);
  if (ms === null) return null;
  const date = new Date(ms);
  const name = zoneName(date, zone);
  return {
    zone: name,
    local: cardReading(date, language, zone),
    localCopy: zonedIso(ms, zone),
    utc: name === "UTC" ? null : cardReading(date, language, "UTC"),
    utcCopy: date.toISOString(),
    unix: String(ms),
  };
}

/**
 * `formatDateTime` followed by the zone's short name, as Render's cron job
 * header dates its last successful run (`LL [at] LT z`, w3/m643): "November
 * 21, 2026 at 9:00 AM PST"; other languages keep their own date and name the
 * zone their own way ("21. November 2026 um 09:00 MEZ"). The runtime's zone
 * (UTC in the SSR container), so render it only after hydration, through
 * `LocalDateTimeWithZone`. Null when `iso` is missing or unparseable.
 */
export function formatDateTimeWithZone(
  iso: string | null | undefined,
  language?: string,
): string | null {
  const text = formatDateTime(iso, language);
  const ms = parseInstant(iso);
  if (text === null || ms === null) return null;
  const tag = LANGUAGE_TAGS[dashboardLanguage(language)];
  const zone = zoneName(new Date(ms), runtimeZone(), tag);
  return zone ? `${text} ${zone}` : text;
}

/**
 * How long after `since` the instant `iso` is, in Render's duration format
 * (`formatDuration`): a log line's From previous and From start. Null when
 * either is missing or unparseable.
 */
export function formatInstantGap(
  iso: string | null | undefined,
  since: string | null | undefined,
  language?: string,
): string | null {
  const at = parseInstant(iso);
  const from = parseInstant(since);
  if (at === null || from === null) return null;
  return formatDuration(at - from, language);
}

function parseInstant(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const ms = Date.parse(iso);
  return Number.isNaN(ms) ? null : ms;
}

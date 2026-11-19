import { perLanguage } from "./language";

const clock = perLanguage({
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: true,
});
const timeOfDay = perLanguage({
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
  hour12: true,
});

/**
 * Render's log line clock in the dashboard language: "03:36:01 AM", or
 * "上午03:36:01" in zh, never the browser's locale. `instant` is an RFC3339
 * stamp or epoch milliseconds. The reading follows the runtime's timezone, so
 * it belongs in client-rendered rows only. An empty or unparseable stamp
 * renders blank rather than "Invalid Date".
 */
export function formatClockTime(
  instant: string | number,
  language?: string,
): string {
  return formatWith(clock(language), instant);
}

/**
 * A chart point's time of day: "7:06:00 AM", "上午7:06:00". Client-only, as
 * `formatClockTime`.
 */
export function formatTimeOfDay(
  instant: string | number,
  language?: string,
): string {
  return formatWith(timeOfDay(language), instant);
}

const shortDateTime = perLanguage({
  year: "numeric",
  month: "numeric",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
  hour12: true,
});
const mediumDateTime = perLanguage({ dateStyle: "medium", timeStyle: "short" });
const axisTime = perLanguage({
  hour: "numeric",
  minute: "2-digit",
  hour12: true,
});
const axisDate = perLanguage({
  month: "short",
  day: "numeric",
  hour: "numeric",
  hour12: true,
});
const axisDay = perLanguage({ month: "short", day: "numeric" });

/**
 * A chart's x-axis label: the time of day to the minute ("2:15 PM",
 * "下午2:15"), or the day and hour ("Nov 12, 6 PM", "11月12日 下午6时") when
 * the axis spans more than a day and a half — the day alone repeated across
 * a 48-hour axis ("Nov 17 · Nov 17 · Nov 17"). Client-only, as
 * `formatClockTime`.
 */
export function formatAxisTime(
  instant: number,
  spanMs: number,
  language?: string,
  /** The axis's tick step: a whole-day step labels the day alone. */
  stepMs = 0,
): string {
  const day = 24 * 60 * 60 * 1000;
  return formatWith(
    stepMs >= day
      ? axisDay(language)
      : spanMs > 1.5 * day
        ? axisDate(language)
        : axisTime(language),
    instant,
  );
}

/** A numeric date and time to the second: "11/12/2026, 3:39:00 PM",
 *  "2026/11/12 下午3:39:00". Client-only. */
export function formatShortDateTime(
  instant: string | number,
  language?: string,
): string {
  return formatWith(shortDateTime(language), instant);
}

/** A compact date and time: "Nov 12, 2026, 7:06 AM", "2026/11/12 上午7:06".
 *  Client-only. */
export function formatMediumDateTime(
  instant: string | number,
  language?: string,
): string {
  return formatWith(mediumDateTime(language), instant);
}

function formatWith(format: Intl.DateTimeFormat, instant: string | number) {
  if (instant === "") return "";
  const date = new Date(instant);
  return Number.isNaN(date.getTime()) ? "" : format.format(date);
}

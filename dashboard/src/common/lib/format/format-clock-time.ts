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

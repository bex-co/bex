import { LANGUAGE_TAGS } from "@/i18n/config";
import { dashboardLanguage } from "./language";

const SECOND_MS = 1000;
const MINUTE_MS = 60 * SECOND_MS;
const HOUR_MS = 60 * MINUTE_MS;

/**
 * A span as Render's dashboard writes it (its `utils-*.js` duration format,
 * behind a cron run's duration and a log line's From previous and From start):
 * whole milliseconds under 100 ms ("42ms"), tenths of a second under a minute
 * ("14.5s"), then "1m 5s", "2h 3m" and "1d 4h", dropping a zero second part
 * ("3m"). A negative span reads "-" and its size. The decimal is the
 * dashboard language's ("14,5s" in de); the units stay Render's.
 */
export function formatDuration(ms: number, language?: string): string {
  if (ms < 0) return `-${formatDuration(-ms, language)}`;
  const tag = LANGUAGE_TAGS[dashboardLanguage(language)];
  if (ms < 1) {
    return `${ms.toLocaleString(tag, { maximumSignificantDigits: 1, useGrouping: false })}ms`;
  }
  if (Math.round(ms) < 100) return `${Math.round(ms)}ms`;
  const tenths = Math.round(ms / 100);
  if (tenths < 600) {
    return `${(tenths / 10).toLocaleString(tag, { maximumFractionDigits: 1 })}s`;
  }
  const seconds = Math.round(ms / SECOND_MS);
  if (seconds < 3600)
    return pair(Math.floor(seconds / 60), "m", seconds % 60, "s");
  if (seconds < 86_400) {
    const minutes = Math.round(ms / MINUTE_MS);
    return pair(Math.floor(minutes / 60), "h", minutes % 60, "m");
  }
  const hours = Math.round(ms / HOUR_MS);
  return pair(Math.floor(hours / 24), "d", hours % 24, "h");
}

function pair(
  major: number,
  majorUnit: string,
  minor: number,
  minorUnit: string,
) {
  return minor === 0
    ? `${major}${majorUnit}`
    : `${major}${majorUnit} ${minor}${minorUnit}`;
}

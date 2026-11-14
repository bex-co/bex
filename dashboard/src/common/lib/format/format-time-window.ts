import { perLanguage } from "./language";

const window = perLanguage({
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
});

/**
 * A custom time window in the dashboard language, the parts both ends share
 * said once: "Nov 12, 3:39:00 PM – 3:39:20 PM", "11/12 下午3:39:00 –
 * 下午3:39:20". The reading follows the runtime's timezone, so render it only
 * after hydration. Null when either end is missing or unparseable.
 */
export function formatTimeWindow(
  startIso: string,
  endIso: string,
  language?: string,
): string | null {
  const start = Date.parse(startIso);
  const end = Date.parse(endIso);
  if (Number.isNaN(start) || Number.isNaN(end)) return null;
  return window(language).formatRange(start, end);
}

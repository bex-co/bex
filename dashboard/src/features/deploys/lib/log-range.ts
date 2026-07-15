// The deploy page log viewer's `?r=<range>` URL param (w9/003): a shared link
// reopens the same RELATIVE time window (Render's own deploy-page `?r=`
// behavior). Absent means the default — the deploy's own createdAt..finishedAt
// window — which is why there is no "deploy" member here: clearing the param
// IS selecting the deploy window.

export const LOG_RANGES = ["15m", "1h", "6h", "24h", "7d"] as const;

export type LogRange = (typeof LOG_RANGES)[number];

const RANGE_MS: Record<LogRange, number> = {
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
};

// Feature-prefixed i18n keys, one per range (single-brace interpolation would
// force pluralized unit strings; five explicit keys read better in both
// locales).
export const RANGE_LABEL_KEYS: Record<LogRange, string> = {
  "15m": "deploys.logRangeLast15m",
  "1h": "deploys.logRangeLast1h",
  "6h": "deploys.logRangeLast6h",
  "24h": "deploys.logRangeLast24h",
  "7d": "deploys.logRangeLast7d",
};

/** Validates an unknown search-param value; anything unrecognized => undefined
 *  (the deploy window), never a crash on a hand-edited URL. */
export function parseLogRange(value: unknown): LogRange | undefined {
  return LOG_RANGES.find((r) => r === value);
}

/** The RFC3339 start of a relative range ending now. */
export function rangeStart(range: LogRange, now = Date.now()): string {
  return new Date(now - RANGE_MS[range]).toISOString();
}

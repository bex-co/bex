// The restore dialog's "Point in time" is a `datetime-local` value: the
// viewer's wall time, no zone. The recovery window beside it is shown in the
// same local zone (w6/m107), so the picked value is converted as local time,
// never labelled UTC (w21/020).

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

/** An instant as a `datetime-local` value ("YYYY-MM-DDTHH:mm") in the viewer's zone. */
export function toLocalInput(ms: number): string {
  const d = new Date(ms);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** The instant a `datetime-local` value denotes in the viewer's zone, or null. */
export function fromLocalInput(value: string): number | null {
  if (!value) return null;
  const ms = new Date(value).getTime();
  return Number.isNaN(ms) ? null : ms;
}

/**
 * How far before now a restore target must lie. Render: "You can't restore to
 * a time that's within ten minutes of the current time", and bex-api refuses
 * one with RECOVERY_TARGET_TOO_RECENT (w7/m453).
 */
export const RECOVERY_MIN_AGE_MS = 10 * 60 * 1000;

export interface RestoreBounds {
  /** `min`/`max` attribute values, at the minute the panel displays. */
  min: string | undefined;
  max: string;
  earliestMs: number | null;
  /** The latest restorable instant: ten minutes before `nowMs`. */
  latestMs: number;
}

/**
 * The picker's bounds at `nowMs`: from the earliest restore point bex-api
 * reports (already floored at the workspace plan's window, w7/m453) to ten
 * minutes before now.
 */
export function restoreBounds(
  earliest: string | null | undefined,
  nowMs: number,
): RestoreBounds {
  const earliestMs = earliest ? Date.parse(earliest) : NaN;
  const hasEarliest = !Number.isNaN(earliestMs);
  const latestMs = nowMs - RECOVERY_MIN_AGE_MS;
  return {
    min: hasEarliest ? toLocalInput(earliestMs) : undefined,
    max: toLocalInput(latestMs),
    earliestMs: hasEarliest ? earliestMs : null,
    latestMs,
  };
}

/**
 * The RFC3339 target for a picked value, or why it cannot be one. A pick at
 * the earliest point's displayed minute is clamped to the exact earliest
 * instant (the field has minute precision, the window has seconds).
 */
export function restoreTarget(
  value: string,
  bounds: RestoreBounds,
):
  | { ok: true; iso: string | undefined }
  | { ok: false; reason: "outOfWindow" } {
  if (!value) return { ok: true, iso: undefined };
  const ms = fromLocalInput(value);
  if (ms === null) return { ok: false, reason: "outOfWindow" };
  if (ms > bounds.latestMs) return { ok: false, reason: "outOfWindow" };
  let target = ms;
  if (bounds.earliestMs !== null && ms < bounds.earliestMs) {
    if (value !== bounds.min) return { ok: false, reason: "outOfWindow" };
    target = bounds.earliestMs;
  }
  return {
    ok: true,
    iso: new Date(target).toISOString().replace(".000Z", "Z"),
  };
}

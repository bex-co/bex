export interface RangePreset {
  id: "30m" | "1h" | "4h" | "12h" | "24h" | "2d" | "7d" | "14d" | "30d";
  spanSeconds: number;
  resolutionSeconds: number;
}

// Render's preset ladder, captured live 2026-07-17 (w5/m42): 30 min / hour /
// 4 hours / 12 hours / 24 hours / 2 days / 7 days / 14 days, plus "Last 30 days"
// (w5/m56 — Render plan-gates it; bex offers it ungated). Render's "Custom" is a
// separate absolute range (see CustomRange). Resolution scales with span so a
// series stays readable (~120 points, not thousands) while still resolving
// fine-grained enough to show shape. Display labels live in range-select.tsx's
// RANGE_LABEL_KEYS (i18n). Ranges beyond Prometheus's retention
// (deploy/gitops/base/prometheus.yaml) simply show the retained window — same as
// Render past its own horizon.
export const RANGE_PRESETS: RangePreset[] = [
  { id: "30m", spanSeconds: 30 * 60, resolutionSeconds: 15 },
  { id: "1h", spanSeconds: 60 * 60, resolutionSeconds: 30 },
  { id: "4h", spanSeconds: 4 * 60 * 60, resolutionSeconds: 120 },
  { id: "12h", spanSeconds: 12 * 60 * 60, resolutionSeconds: 300 },
  { id: "24h", spanSeconds: 24 * 60 * 60, resolutionSeconds: 720 },
  { id: "2d", spanSeconds: 2 * 24 * 60 * 60, resolutionSeconds: 1440 },
  { id: "7d", spanSeconds: 7 * 24 * 60 * 60, resolutionSeconds: 5040 },
  { id: "14d", spanSeconds: 14 * 24 * 60 * 60, resolutionSeconds: 10080 },
  { id: "30d", spanSeconds: 30 * 24 * 60 * 60, resolutionSeconds: 21600 },
];

export const DEFAULT_RANGE_PRESET = RANGE_PRESETS[3]; // "12h" — Render's own default

export type RangePresetID = RangePreset["id"];

/** Returns the supported relative range named by a URL value, or null. */
export function parseRangePreset(value: unknown): RangePreset | null {
  if (typeof value !== "string") return null;
  return RANGE_PRESETS.find((preset) => preset.id === value) ?? null;
}

/**
 * An absolute [startTime, endTime] window — Render's "Custom" range (w5/m56).
 * Unlike a preset it never slides with wall-clock time; the picker resolves the
 * resolution from the span so a custom chart stays as readable as a preset one.
 */
export interface CustomRange {
  id: "custom";
  startTime: string; // ISO
  endTime: string; // ISO
  resolutionSeconds: number;
}

/** A range selection: a relative preset or an absolute custom window. */
export type RangeSelection = RangePreset | CustomRange;

export function isCustomRange(range: RangeSelection): range is CustomRange {
  return range.id === "custom";
}

// The widest custom window bex offers by default — 30 days, matching
// BEX_MAX_QUERY_HOURS' default (root CLAUDE.md). A window beyond the backend's
// actual cap still fails honestly with the over-window 400; this client guard
// just keeps the common case from round-tripping a doomed query.
export const MAX_CUSTOM_RANGE_HOURS = 720;

/** ~120 buckets for any span, floored at 1s so a tiny window still steps. */
export function resolutionForSpan(spanSeconds: number): number {
  return Math.max(1, Math.round(spanSeconds / 120));
}

export type CustomRangeError = "order" | "tooLong";

/**
 * Builds an absolute custom range from two ISO instants, or an error token the
 * caller maps to a message: the end must be after the start, and the window may
 * not exceed MAX_CUSTOM_RANGE_HOURS.
 */
export function makeCustomRange(
  startTime: string,
  endTime: string,
): CustomRange | CustomRangeError {
  const start = new Date(startTime).getTime();
  const end = new Date(endTime).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return "order";
  }
  const spanSeconds = (end - start) / 1000;
  if (spanSeconds > MAX_CUSTOM_RANGE_HOURS * 3600) return "tooLong";
  return {
    id: "custom",
    startTime: new Date(start).toISOString(),
    endTime: new Date(end).toISOString(),
    resolutionSeconds: resolutionForSpan(spanSeconds),
  };
}

/** Reconstructs a stored custom range from ISO start/end, or null if invalid. */
export function parseCustomRange(
  startTime: unknown,
  endTime: unknown,
): CustomRange | null {
  if (typeof startTime !== "string" || typeof endTime !== "string") return null;
  const made = makeCustomRange(startTime, endTime);
  return typeof made === "string" ? null : made;
}

/**
 * The shared URL shape for a persisted range selection (w6/065): `range` names
 * a preset id or `"custom"`, with `rangeStart`/`rangeEnd` carrying a custom
 * window's ISO bounds. This is the same key vocabulary the Logs page's
 * LogSearch uses (features/logs/lib/log-search.ts) — minus its Logs-specific
 * `r` Render alias — so Metrics and the datastore log viewers stay
 * deep-linkable in one spelling.
 */
export interface RangeSearch {
  range?: RangePresetID | "custom";
  rangeStart?: string;
  rangeEnd?: string;
}

/**
 * Validates the shared range keys off a raw URL search object. Malformed or
 * unknown values drop out entirely (a bad link renders the surface's default
 * range), and a custom range is kept only when both bounds reconstruct a valid
 * window — mirroring parseLogSearch's range handling.
 */
export function parseRangeSearch(search: Record<string, unknown>): RangeSearch {
  if (search.range === "custom") {
    const start =
      typeof search.rangeStart === "string" ? search.rangeStart : undefined;
    const end =
      typeof search.rangeEnd === "string" ? search.rangeEnd : undefined;
    if (start && end && parseCustomRange(start, end)) {
      return { range: "custom", rangeStart: start, rangeEnd: end };
    }
    return {};
  }
  const preset = parseRangePreset(search.range);
  return preset ? { range: preset.id } : {};
}

/** Restores the selection a validated RangeSearch names, else the caller's
 *  surface default (absent/dropped params keep today's behavior). */
export function rangeFromSearch(
  search: RangeSearch,
  defaultRange: RangeSelection,
): RangeSelection {
  if (search.range === "custom" && search.rangeStart && search.rangeEnd) {
    const custom = parseCustomRange(search.rangeStart, search.rangeEnd);
    if (custom) return custom;
  }
  return parseRangePreset(search.range) ?? defaultRange;
}

/**
 * Serializes a selection into the shared URL keys. Every key is written
 * explicitly — `undefined` for the non-custom case — because the router
 * retains params an update merely omits (w7/m42), so switching custom → preset
 * must actively clear the stale bounds.
 */
export function rangeToSearch(range: RangeSelection): RangeSearch {
  return {
    range: range.id,
    rangeStart: isCustomRange(range) ? range.startTime : undefined,
    rangeEnd: isCustomRange(range) ? range.endTime : undefined,
  };
}

// A relative window is just a span + a bucket size — the preset `id` is a
// picker/URL concern, so consumers with a fixed window (e.g. the Scaling
// page's 48h Recent Metrics, w7/m43) can pass a bare window without minting a
// fake preset id.
export type RangeWindow = Pick<
  RangePreset,
  "spanSeconds" | "resolutionSeconds"
>;

/** Resolves a window into the startTime/endTime/resolutionSeconds query args. */
export function resolveRange(preset: RangeWindow, now: Date) {
  const end = now.toISOString();
  const start = new Date(
    now.getTime() - preset.spanSeconds * 1000,
  ).toISOString();
  return {
    startTime: start,
    endTime: end,
    resolutionSeconds: preset.resolutionSeconds,
  };
}

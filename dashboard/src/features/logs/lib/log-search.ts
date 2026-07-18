import {
  parseRangePreset,
  type RangePreset,
  type RangePresetID,
} from "@/features/metrics/lib/range";
import {
  EMPTY_LOG_FILTERS,
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
  STRUCTURED_FILTER_KEYS,
  type LogFilters,
} from "../types";

// The Logs tab's URL search state (w7/m42): every filter round-trips through
// the query string so a filtered view is shareable, bookmarkable, and
// reload-stable. Default/empty values are omitted so an unfiltered view keeps a
// clean URL (no `?type=all` noise).
export interface LogSearch {
  range?: RangePresetID;
  type?: typeof LOG_TYPE_APP | typeof LOG_TYPE_REQUEST;
  level?: string;
  method?: string;
  statusCode?: string;
  instance?: string;
  path?: string;
  text?: string;
  /** Present only when the live tail is toggled off (`?live=0`); absent = on. */
  live?: 0;
  /**
   * Consumed-alias tombstones: the router carries raw query params it doesn't
   * know about into every outbound write (`{...location.search, ...validated}`),
   * so a folded `t`/`r` alias would linger in the URL forever unless the
   * validated result explicitly overrides it with `undefined` (w7/m42/t003).
   */
  t?: undefined;
  r?: undefined;
}

// The string-valued filter keys = the structured fields + free-text search.
const TEXT_KEYS = [...STRUCTURED_FILTER_KEYS, "text"] as const;
// Every canonical key, for the equality guard below.
const SEARCH_KEYS = ["range", "type", ...TEXT_KEYS, "live"] as const;

// Render's `r` tokens without an exact bex preset (Render's grammar is
// 15m|1h|6h|24h|7d — the captured deploy-page contract, see
// features/deploys/lib/log-range.ts) map onto the nearest bex preset; the
// overlapping tokens (1h/24h/7d) and bex's own ids parse directly. bex's
// retired preset ids (3h/6h/1d, pre-w5/m42) are deliberately not aliased —
// an old bookmark degrades to the default range rather than a guess.
const RENDER_RANGE_ALIASES: Record<string, RangePresetID> = {
  "15m": "30m",
  "6h": "4h",
};

// TanStack Router JSON-parses each query value, so `?statusCode=404` arrives as
// the number 404 — accept it back as a string. Anything else non-string is
// dropped: a malformed link still renders, with defaults.
function str(value: unknown): string | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return typeof value === "string" && value !== "" ? value : undefined;
}

function parseType(value: unknown): LogSearch["type"] {
  // `application` is bex-api's accepted input alias for `app` — honor it in
  // links the same way (docs/ADR010-observability.md § Log types).
  if (value === LOG_TYPE_APP || value === "application") return LOG_TYPE_APP;
  if (value === LOG_TYPE_REQUEST) return LOG_TYPE_REQUEST;
  return undefined;
}

function parseRangeAlias(value: unknown): RangePreset | null {
  if (typeof value !== "string") return null;
  return (
    parseRangePreset(value) ?? parseRangePreset(RENDER_RANGE_ALIASES[value])
  );
}

export function parseLogSearch(search: Record<string, unknown>): LogSearch {
  const out: LogSearch = {};
  // Render's logs URL keys `t` (type) and `r` (range) are accepted as
  // inbound-only aliases so a Render-shaped deep link (`?t=app&r=1h`) prefills
  // the view (w7/m42/t003). bex's canonical keys win when both are present,
  // and the tombstones (see LogSearch) make the aliases leave the URL on the
  // first write, so it self-heals to bex's shape (the w7/m39
  // alias-normalization pattern).
  if ("t" in search) out.t = undefined;
  if ("r" in search) out.r = undefined;
  const range = parseRangePreset(search.range) ?? parseRangeAlias(search.r);
  if (range) out.range = range.id;
  const type = parseType(search.type) ?? parseType(search.t);
  if (type) out.type = type;
  for (const key of TEXT_KEYS) {
    const value = str(search[key]);
    if (value) out[key] = value;
  }
  if (
    search.live === 0 ||
    search.live === "0" ||
    search.live === false ||
    search.live === "false"
  ) {
    out.live = 0;
  }
  return out;
}

// The Logs tab keeps its documented one-hour default query span (bex-api's
// own default) — the Metrics page follows Render's 12-hour default instead
// (w5/m42), so the two tabs deliberately no longer share one default.
export const DEFAULT_LOG_RANGE = parseRangePreset("1h")!;

/** Restores the URL range, falling back to the documented one-hour default. */
export function logRangeFromSearch(search: LogSearch): RangePreset {
  return parseRangePreset(search.range) ?? DEFAULT_LOG_RANGE;
}

/** Restores the filter state named by the URL (absent keys = defaults). */
export function logFiltersFromSearch(search: LogSearch): LogFilters {
  const out = { ...EMPTY_LOG_FILTERS };
  if (search.type) out.type = search.type;
  for (const key of TEXT_KEYS) out[key] = search[key] ?? "";
  return out;
}

/**
 * Serializes filter + live state into URL search keys, omitting defaults so an
 * unfiltered view keeps a clean URL. `range` is owned by the range handler and
 * merged by the route.
 */
export function logFiltersToSearch(
  filters: LogFilters,
  live: boolean,
): Omit<LogSearch, "range"> {
  const out: Omit<LogSearch, "range"> = {};
  if (filters.type === LOG_TYPE_APP || filters.type === LOG_TYPE_REQUEST) {
    out.type = filters.type;
  }
  for (const key of TEXT_KEYS) {
    if (filters[key]) out[key] = filters[key];
  }
  if (!live) out.live = 0;
  return out;
}

/**
 * Whether two validated searches name the same filter state. The route skips
 * navigation when the URL already encodes the reported state — there is
 * nothing to write, and navigating with an unchanged target during hydration
 * re-enters a full document load loop under TanStack Start (observed live,
 * w7/m42). Derived from SEARCH_KEYS so a new filter field can't silently
 * drift out of the guard.
 */
export function logSearchEquals(a: LogSearch, b: LogSearch): boolean {
  return SEARCH_KEYS.every((key) => a[key] === b[key]);
}

/**
 * Explicit `undefined`s for every key this route owns plus the inbound-only
 * `t`/`r` aliases: the router retains raw query params across navigations
 * unless they are explicitly written as `undefined` (verified live, w7/m42 —
 * an omitted key lingers in the URL; stringification drops the undefineds).
 * Spread this first, then the keys to keep. Derived from SEARCH_KEYS so a new
 * filter field can't silently drift out of the clear-set.
 */
export const CLEARED_LOG_SEARCH = Object.fromEntries(
  ["t", "r", ...SEARCH_KEYS].map((key) => [key, undefined]),
) as Partial<Record<"t" | "r" | (typeof SEARCH_KEYS)[number], undefined>>;

import {
  parseRangePreset,
  type RangePreset,
  type RangePresetID,
} from "@/features/metrics/lib/range";
import { defaultStringifySearch } from "@tanstack/react-router";
import {
  EMPTY_LOG_FILTERS,
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
  STRUCTURED_FILTER_KEYS,
  type LogFilters,
  type LogLine,
} from "../types";
import {
  DEFAULT_LOG_RANGE,
  LIVE_TAIL,
  logRangeFromSearch,
  parseLogRangeSearch,
  type LogRange,
  type LogRangeID,
  type LogRangeSearch,
} from "./log-range";

// The Logs tab's URL search state (w7/m42): every filter round-trips through
// the query string so a filtered view is shareable, bookmarkable, and
// reload-stable. Default/empty values are omitted so an unfiltered view keeps a
// clean URL (no `?type=all` noise).
export interface LogSearch extends LogRangeSearch {
  type?: typeof LOG_TYPE_APP | typeof LOG_TYPE_REQUEST;
  level?: string;
  method?: string;
  statusCode?: string;
  instance?: string;
  path?: string;
  host?: string;
  text?: string;
  /** The line a View in context link marks (w3/m528): its key. */
  contextLine?: string;
  /**
   * Consumed-alias tombstones: the router carries raw query params it doesn't
   * know about into every outbound write (`{...location.search, ...validated}`),
   * so a folded `t`/`r` alias would linger in the URL forever unless the
   * validated result explicitly overrides it with `undefined` (w7/m42/t003).
   * `live` is one too: the Live switch's `?live=0` (w3/m537), read now as
   * the paused range it names, which is every range but Live tail.
   */
  t?: undefined;
  r?: undefined;
  live?: undefined;
}

// The string-valued filter keys = the structured fields + free-text search.
const TEXT_KEYS = [...STRUCTURED_FILTER_KEYS, "text"] as const;
// Every canonical key, for the equality guard below.
const SEARCH_KEYS = [
  "range",
  "rangeStart",
  "rangeEnd",
  "type",
  ...TEXT_KEYS,
  "contextLine",
] as const;

/**
 * Explicit `undefined`s for every key this route owns plus the inbound-only
 * `t`/`r` aliases and the retired `live`: the router retains raw query params across navigations
 * unless they are explicitly written as `undefined` (verified live, w7/m42 —
 * an omitted key lingers in the URL; stringification drops the undefineds).
 * Spread this first, then the keys to keep. Derived from SEARCH_KEYS so a new
 * filter field can't silently drift out of the clear-set.
 */
export const CLEARED_LOG_SEARCH = Object.fromEntries(
  ["t", "r", "live", ...SEARCH_KEYS].map((key) => [key, undefined]),
) as Partial<
  Record<"t" | "r" | "live" | (typeof SEARCH_KEYS)[number], undefined>
>;

// Render's `r` tokens without an exact bex preset (Render's grammar is
// 15m|1h|6h|24h|7d — the captured deploy-page contract, see
// features/deploys/lib/log-range.ts) map onto the nearest bex preset; the
// overlapping tokens (1h/24h/7d) and bex's own ids parse directly. A log
// explorer then reads a Metrics-only preset as its nearest log range
// (`toLogRange`, w3/100), so `15m` opens on Last hour. bex's
// retired preset ids (3h/6h/1d, pre-w5/m42) are deliberately not aliased —
// an old bookmark degrades to the default range rather than a guess.
const RENDER_RANGE_ALIASES: Record<string, RangePresetID> = {
  "15m": "30m",
  "6h": "4h",
};

// TanStack Router JSON-parses each query value, so `?statusCode=404` arrives as
// the number 404 — accept it back as a string. Anything else non-string is
// dropped: a malformed link still renders, with defaults.
export function str(value: unknown): string | undefined {
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

export function parseRangeAlias(value: unknown): RangePreset | null {
  if (typeof value !== "string") return null;
  return (
    parseRangePreset(value) ?? parseRangePreset(RENDER_RANGE_ALIASES[value])
  );
}

/** A Render `r` value as a log range id: Render's `live` (w3/m537) or a
 *  preset `parseRangeAlias` reads. */
export function parseLogRangeAlias(value: unknown): LogRangeID | undefined {
  return value === LIVE_TAIL.id ? LIVE_TAIL.id : parseRangeAlias(value)?.id;
}

export function parseLogSearch(search: Record<string, unknown>): LogSearch {
  // Every owned key comes back, undefined when rejected: the router merges this
  // over the raw search, so an omitted `?type=bogus` would reach bex-api.
  //
  // Render's logs URL keys `t` (type) and `r` (range) are accepted as
  // inbound-only aliases so a Render-shaped deep link (`?t=app&r=1h`) prefills
  // the view (w7/m42/t003). bex's canonical keys win when both are present,
  // and the tombstones (see LogSearch) make the aliases leave the URL on the
  // first write, so it self-heals to bex's shape (the w7/m39
  // alias-normalization pattern).
  const out: LogSearch = {
    ...CLEARED_LOG_SEARCH,
    ...parseLogRangeSearch(search),
  };
  if (!out.range && search.range !== "custom") {
    out.range = parseLogRangeAlias(search.r);
  }
  out.type = parseType(search.type) ?? parseType(search.t);
  for (const key of TEXT_KEYS) out[key] = str(search[key]);
  out.contextLine = str(search.contextLine);
  return out;
}

// How far either side of a line View in context opens (Render's ±10 minutes).
const CONTEXT_SPAN_MS = 10 * 60 * 1000;
// A line's key carries its message when the backend gave it no id; a link
// carries it only when it is short enough to be one.
const MAX_CONTEXT_KEY_LENGTH = 512;

/**
 * The search Render's View in context opens (w3/m528): the explorer with no
 * search or filter, a custom range from 10 minutes before the line to 10
 * minutes after it (which never tails), and the line marked. Null for a line
 * without a time, which has no surroundings to open.
 */
export function logContextSearch(line: LogLine): {
  range: "custom";
  rangeStart: string;
  rangeEnd: string;
  contextLine?: string;
} | null {
  const at = Date.parse(line.timestamp);
  if (Number.isNaN(at)) return null;
  return {
    range: "custom",
    rangeStart: new Date(at - CONTEXT_SPAN_MS).toISOString(),
    rangeEnd: new Date(at + CONTEXT_SPAN_MS).toISOString(),
    contextLine:
      line.key.length <= MAX_CONTEXT_KEY_LENGTH ? line.key : undefined,
  };
}

/** A link to `path` with `search`, written as the router writes it. */
export function searchHref(
  path: string,
  search: Record<string, unknown>,
): string {
  return `${path}${defaultStringifySearch(search)}`;
}

/**
 * A service's Shell link for one instance (w3/m528, w4/717; Render's
 * `${servicePath}/shell/${instance}`): the Shell page on that instance while
 * it is one of `running`, or `null` (disabled) once it is not.
 */
export function serviceInstanceShellHref(
  serviceId: string,
  instanceId: string,
  running: ReadonlySet<string>,
): string | null {
  return running.has(instanceId)
    ? searchHref(`/services/${serviceId}/shell`, {
        instance: instanceId,
      })
    : null;
}

/** A service's Logs tab pre-filtered to one instance (w4/717). */
export function serviceInstanceLogsHref(
  serviceId: string,
  instanceId: string,
): string {
  return searchHref(`/services/${serviceId}/logs`, {
    instance: instanceId,
  });
}

/**
 * The service Logs tab's SSH into instance for a line (w3/m528, Render's
 * `${servicePath}/shell/${instance}`): the Shell page on the line's instance
 * while it is one of `running`, `href: null` (disabled) once it is not, and
 * nothing for a line without an instance.
 */
export function serviceLineShell(
  serviceId: string,
  running: ReadonlySet<string>,
): (line: LogLine) => { href: string | null } | null {
  return (line) =>
    line.instance
      ? {
          href: serviceInstanceShellHref(serviceId, line.instance, running),
        }
      : null;
}

/** The service Logs tab's View in context link for a line (w3/m528). */
export function serviceLogContextHref(
  serviceId: string,
  line: LogLine,
): string | undefined {
  const context = logContextSearch(line);
  return context
    ? searchHref(`/services/${serviceId}/logs`, context)
    : undefined;
}

/** Restores the URL range (Live tail, a preset or a custom window), else
 *  Last hour, as Render's service Logs opens with no `r` (w3/m537). The
 *  Metrics page follows Render's 12-hour default instead (w5/m42). */
export function serviceLogRange(search: LogSearch): LogRange {
  return logRangeFromSearch(search, DEFAULT_LOG_RANGE);
}

/** Restores the filter state named by the URL (absent keys = defaults). */
export function logFiltersFromSearch(search: LogSearch): LogFilters {
  const out = { ...EMPTY_LOG_FILTERS };
  if (search.type) out.type = search.type;
  for (const key of TEXT_KEYS) out[key] = search[key] ?? "";
  return out;
}

/**
 * Serializes filter state into URL search keys, omitting defaults so an
 * unfiltered view keeps a clean URL. `range` is owned by the range handler and
 * merged by the route.
 */
export function logFiltersToSearch(
  filters: LogFilters,
): Omit<LogSearch, "range" | "rangeStart" | "rangeEnd"> {
  const out: Omit<LogSearch, "range" | "rangeStart" | "rangeEnd"> = {};
  if (filters.type === LOG_TYPE_APP || filters.type === LOG_TYPE_REQUEST) {
    out.type = filters.type;
  }
  for (const key of TEXT_KEYS) {
    if (filters[key]) out[key] = filters[key];
  }
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

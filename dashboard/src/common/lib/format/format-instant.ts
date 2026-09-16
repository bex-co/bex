import { formatDistanceStrict } from "date-fns";
import { zhCN } from "date-fns/locale/zh-CN";

export interface TimeAgoOptions {
  /**
   * The reference clock in epoch ms. Injected rather than read here so the
   * output is deterministic under test and so one page-level ticker
   * (`useNow`) can drive every row instead of each row reading its own clock.
   */
  now: number;
  /** The active i18n language ("en", "zh") — picks the unit words. */
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
    locale: language?.toLowerCase().startsWith("zh") ? zhCN : undefined,
  });
}

export interface InstantDetails {
  /** "September 15, 2026 at 9:31:22 AM PDT" — the viewer's own clock. */
  local: string;
  /** "September 15, 2026 at 4:31:22 PM UTC". */
  utc: string;
  /** Unix seconds, "1789464682" — the value a log query or API call wants. */
  unix: string;
}

// Deliberately the same fixed en-US calendar text as `formatDateTime` (month
// names, "at", comma placement) so the two readings of one instant line up;
// `long` adds seconds and the zone name that the row's short form omits.
const LOCAL_FORMAT = new Intl.DateTimeFormat("en-US", {
  dateStyle: "long",
  timeStyle: "long",
});
const UTC_FORMAT = new Intl.DateTimeFormat("en-US", {
  dateStyle: "long",
  timeStyle: "long",
  timeZone: "UTC",
});

/**
 * The exact instant behind a relative time, three ways (local, UTC, Unix) —
 * the body of `InstantTooltip`. The local reading follows the *runtime's*
 * timezone (UTC in the SSR container), so it belongs only in client-side
 * output that is never part of the hydrated markup, such as a tooltip that
 * mounts on hover. Null when `iso` is missing or unparseable.
 */
export function formatInstantDetails(
  iso: string | null | undefined,
): InstantDetails | null {
  const ms = parseInstant(iso);
  if (ms === null) return null;
  const date = new Date(ms);
  return {
    local: LOCAL_FORMAT.format(date),
    utc: UTC_FORMAT.format(date),
    unix: String(Math.floor(ms / 1000)),
  };
}

function parseInstant(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const ms = Date.parse(iso);
  return Number.isNaN(ms) ? null : ms;
}

import type { KeyValuePagePath } from "@/features/keyvalue/components/key-value-nav";
import { splatParts } from "@/common/lib/render-alias";
import {
  metricsPeriodSearch,
  type RangePresetID,
} from "@/features/metrics/lib/range";

/**
 * Render's anchors for an Info card that bex's Info page gives another id:
 * Render's Upgrade instance link is `/info#compute`, and bex's plan picker is
 * its `#plan` card (w3/m649).
 */
export const KEY_VALUE_HASH_ALIASES: Readonly<Record<string, string>> = {
  compute: "plan",
};

/** Where a sub-path of `/keyvalue/<id>/` with no page of its own lands. */
export interface KeyValueSubpathLanding {
  /** The page; null is the bare URL, Info. */
  page: KeyValuePagePath | null;
  /** The card to scroll to, unless the link carries a hash of its own. */
  hash?: string;
  /** Search keys the landing sets over the link's own query. */
  search?: { range: RangePresetID };
}

/**
 * Render's sub-paths that bex has no page for, sent where their control is
 * (docs/render-artifacts/key-value.md § Pages): `/plan`, Render's full-page
 * plan chooser, lands on Info's plan card, bex's plan picker;
 * `/metrics/<period>` is Render's other Metrics route, on the window its
 * period table maps the period to (w3/m707). Anything else lands on Info, as
 * Render's `*` route shows Info.
 */
export function keyValueSubpathLanding(
  splat: string | undefined,
): KeyValueSubpathLanding {
  const [first, period] = splatParts(splat);
  switch (first) {
    case "plan":
      return { page: null, hash: "plan" };
    case "metrics":
      return { page: "/metrics", search: metricsPeriodSearch(period) };
    default:
      return { page: null };
  }
}

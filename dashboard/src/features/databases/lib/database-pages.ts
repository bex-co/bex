import { useLayoutEffect } from "react";
import { useLocation, useNavigate } from "@tanstack/react-router";
import {
  databasePageRoute,
  type DatabasePagePath,
} from "@/features/databases/components/database-nav";
import { splatParts } from "@/common/lib/render-alias";
import {
  metricsPeriodSearch,
  type RangePresetID,
} from "@/features/metrics/lib/range";

/**
 * The old single page's section anchors whose cards left Info when it split
 * into Render's pages (w3/m641), and Render's `#storage` (w3/m656), with the
 * page each card lives on now. The
 * other anchors (`#metadata`, `#connection`, `#access-control`,
 * `#sql-console`, `#danger-zone`) name cards still on Info.
 */
const MOVED_ANCHORS: Readonly<Record<string, DatabasePagePath>> = {
  "high-availability": "/plan",
  plan: "/plan",
  // Render's anchor for the storage control, on Plan (w3/m656).
  storage: "/plan",
  metrics: "/metrics",
  insights: "/metrics",
  apps: "/apps",
  recovery: "/recovery",
};

/** The page an old Overview anchor's card moved to, or null if it is Info's. */
export function movedAnchorPage(hash: string): DatabasePagePath | null {
  const id = hash.replace(/^#/, "");
  return Object.hasOwn(MOVED_ANCHORS, id) ? MOVED_ANCHORS[id] : null;
}

/** Where a sub-path of `/databases/<id>/` with no page of its own lands. */
export interface DatabaseSubpathLanding {
  /** The page; null is the bare URL, Info. */
  page: DatabasePagePath | null;
  /** The section to scroll to, unless the link carries a hash of its own. */
  hash?: string;
  /** Search keys the landing sets over the link's own query. */
  search?: { range: RangePresetID };
}

/**
 * Render's sub-paths that bex has no page for, sent where their control is
 * (docs/render-artifacts/postgres-dashboard.md): `/upgrade` replaces itself
 * with `/plan` on Render too; `/high-availability-upgrade`, which a primary's
 * Render also sends to `/plan`, lands on Plan's High Availability card;
 * `/metrics/<period>` is Render's other Metrics route, on the window its
 * period table maps the period to (w3/m707). Anything else lands on Info, as
 * Render's `*` route shows Info. `/version-upgrade` is a page of its own
 * since w3/m676.
 */
export function databaseSubpathLanding(
  splat: string | undefined,
): DatabaseSubpathLanding {
  const [first, period] = splatParts(splat);
  switch (first) {
    case "upgrade":
      return { page: "/plan" };
    case "high-availability-upgrade":
      return { page: "/plan", hash: "high-availability" };
    case "metrics":
      return { page: "/metrics", search: metricsPeriodSearch(period) };
    default:
      return { page: null };
  }
}

/**
 * On a full load the router does not run beforeLoad again on the client, and
 * a hash never reaches the server, so the bare URL's redirect for an old
 * anchor cannot fire there. The Info page runs this instead: before the
 * browser paints the hydrated page, a moved card's anchor goes on to its page,
 * the hash kept.
 */
export function useMovedAnchorRedirect(databaseId: string) {
  const navigate = useNavigate();
  const hash = useLocation({ select: (location) => location.hash });
  const page = movedAnchorPage(hash);
  useLayoutEffect(() => {
    if (!page) return;
    void navigate({
      to: databasePageRoute(page),
      params: { databaseId },
      hash,
      replace: true,
    });
  }, [page, hash, databaseId, navigate]);
}

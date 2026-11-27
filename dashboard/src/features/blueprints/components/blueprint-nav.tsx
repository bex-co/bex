import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  resourcePageNav,
  resourcePageNavItem,
} from "@/common/components/dashboard-layout/resource-page-nav";
import { Boxes, RefreshCcw, Settings } from "lucide-react";

// A Blueprint's pages, as Render's sidebar lists them (w3/m677,
// docs/render-artifacts/blueprint-pages.md): Resources, Syncs, Settings, in
// one group with no label. No entry depends on a permission, as on Render.
//
// One source of truth: the Blueprint sidebar
// (common/components/dashboard-layout/blueprint-sidebar.tsx) draws these
// groups and the detail layout names each page with its entry's label.

/** A page's path under the Blueprint, e.g. `/syncs`. */
export type BlueprintPagePath = "/resources" | "/syncs" | "/settings";

/** A page's route, typed for links and redirects. */
export function blueprintPageRoute<P extends BlueprintPagePath>(page: P) {
  return `/blueprints/$blueprintId${page}` as const;
}

// A sync's own pages (w3/m678): `/sync/<syncId>`, and `/sync` for the plan
// Manual sync hands over.
const SYNC_PAGES = ["/sync", "/sync/*"] as const;

// Resources is also the bare URL's page (Render's `hrefPatterns` name
// `/blueprint/:id`), and Syncs the open page on a sync's own URL
// (`/blueprint/:id/sync/:syncId`).
const RESOURCES = resourcePageNavItem(
  blueprintPageRoute("/resources"),
  "blueprints.navResources",
  Boxes,
  [""],
);
const SYNCS = resourcePageNavItem(
  blueprintPageRoute("/syncs"),
  "blueprints.navSyncs",
  RefreshCcw,
  SYNC_PAGES,
);
const SETTINGS = resourcePageNavItem(
  blueprintPageRoute("/settings"),
  "blueprints.navSettings",
  Settings,
);

export const BLUEPRINT_NAV_GROUPS: readonly SidebarNavGroup[] = [
  { items: [RESOURCES, SYNCS, SETTINGS] },
];

const NAV = resourcePageNav("/blueprints/$blueprintId", BLUEPRINT_NAV_GROUPS);

/**
 * Whether the sidebar entry `to` is the page `pathname` is on. `blueprintId`
 * is the id as the URL spells it (a `blp-` bookmark keeps its spelling).
 */
export function isBlueprintNavItemActive(
  pathname: string,
  blueprintId: string,
  to: string,
): boolean {
  return NAV.isActive(pathname, blueprintId, to);
}

/**
 * Whether `pathname` is a sync's page, `/sync` or `/sync/<syncId>` (w3/m678):
 * Syncs is its open entry, and its heading is the sync's own. `blueprintId`
 * is the id as the URL spells it.
 */
export function isBlueprintSyncPage(
  pathname: string,
  blueprintId: string,
): boolean {
  return NAV.isOn(pathname, blueprintId, SYNC_PAGES);
}

/**
 * The open page's sidebar label, which is also its heading. Null off the
 * Blueprint's pages.
 */
export function blueprintPageLabelKey(
  pathname: string,
  blueprintId: string,
): SidebarNavItem["labelKey"] | null {
  return NAV.labelKey(pathname, blueprintId);
}

import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import { filterNavGroups } from "@/common/components/dashboard-layout/dashboard-nav";
import {
  AppWindow,
  ChartNoAxesCombined,
  History,
  Info,
  ScrollText,
  SlidersHorizontal,
} from "lucide-react";

// A Postgres database's pages, as Render's sidebar lists them (w3/m641,
// docs/render-artifacts/postgres-dashboard.md): Info, then **Monitor** (Logs,
// Metrics) and **Manage** (Plan, Apps, Recovery). Render's Sharing page is a
// legacy page behind a flag and stays out; bex has no replica pages (a read
// replica belongs to its primary), so the primary's sidebar is the only one.
//
// One source of truth: the database sidebar
// (common/components/dashboard-layout/database-sidebar.tsx) draws these groups
// and the detail layout names each page with its entry's label.

/** A page's path under the database, e.g. `/recovery`. */
export type DatabasePagePath =
  "/info" | "/logs" | "/metrics" | "/plan" | "/apps" | "/recovery";

/** A page's route, typed for links and redirects. */
export function databasePageRoute<P extends DatabasePagePath>(page: P) {
  return `/databases/$databaseId${page}` as const;
}

interface DatabaseNavItem extends SidebarNavItem {
  /** Other paths under the database on which this entry is the open page. */
  alsoActiveOn: readonly string[];
}

function navItem(
  page: DatabasePagePath,
  labelKey: SidebarNavItem["labelKey"],
  icon: SidebarNavItem["icon"],
  alsoActiveOn: readonly string[] = [],
): DatabaseNavItem {
  return { labelKey, to: databasePageRoute(page), icon, alsoActiveOn };
}

// Info is also the bare URL's page, as on Render (`hrefPatterns: [/d/<id>]`).
// Plan is also the open page on Render's plan sub-pages, which land on it.
const INFO = navItem("/info", "databases.navInfo", Info, [""]);
const LOGS = navItem("/logs", "databases.navLogs", ScrollText);
const METRICS = navItem(
  "/metrics",
  "databases.navMetrics",
  ChartNoAxesCombined,
);
const PLAN = navItem("/plan", "databases.navPlan", SlidersHorizontal, [
  "/upgrade",
  "/high-availability-upgrade",
]);
const APPS = navItem("/apps", "databases.navApps", AppWindow);
const RECOVERY = navItem("/recovery", "databases.navRecovery", History);

const GROUPS: readonly SidebarNavGroup[] = [
  { items: [INFO] },
  { labelKey: "common.navMonitorGroup", items: [LOGS, METRICS] },
  { labelKey: "common.navManageGroup", items: [PLAN, APPS, RECOVERY] },
];

/**
 * The database's nav groups. `logsDenied` is a confirmed can_view_logs denial
 * (a Viewer's or a Billing member's, w9/145) and drops Logs, as Render drops
 * it without `viewDatabaseLogs`; `metricsDenied` is a confirmed
 * can_view_activity denial (a Billing member's, w9/m621) and drops Metrics,
 * as Render drops it without `viewDatabaseMetrics`. A group left empty is
 * dropped. While capabilities are still checking both are false, so an
 * allowed member's nav never shifts.
 */
export function databaseNavGroups({
  logsDenied = false,
  metricsDenied = false,
}: { logsDenied?: boolean; metricsDenied?: boolean } = {}): SidebarNavGroup[] {
  return filterNavGroups(
    [...GROUPS],
    (item) =>
      !(logsDenied && item === LOGS) && !(metricsDenied && item === METRICS),
  );
}

/** The path under `/databases/<id>` that `pathname` names: `""` for the bare URL. */
function pagePath(pathname: string, databaseId: string): string | null {
  const base = `/databases/${databaseId}`;
  const path = pathname.replace(/\/+$/, "");
  if (path === base) return "";
  return path.startsWith(`${base}/`) ? path.slice(base.length) : null;
}

function itemFor(path: string): DatabaseNavItem | undefined {
  for (const group of GROUPS) {
    for (const item of group.items as DatabaseNavItem[]) {
      if (
        item.to === `/databases/$databaseId${path}` ||
        item.alsoActiveOn.includes(path)
      ) {
        return item;
      }
    }
  }
  return undefined;
}

/** Whether the sidebar entry `to` is the page `pathname` is on. */
export function isDatabaseNavItemActive(
  pathname: string,
  databaseId: string,
  to: string,
): boolean {
  const path = pagePath(pathname, databaseId);
  return path !== null && itemFor(path)?.to === to;
}

/**
 * The open page's sidebar label, which is also its heading, so the loading
 * frame and the loaded page carry the same words (on a phone the sidebar is in
 * a drawer, and nothing else on the page would say which page is open). Null
 * off the database's pages.
 */
export function databasePageLabelKey(
  pathname: string,
  databaseId: string,
): SidebarNavItem["labelKey"] | null {
  const path = pagePath(pathname, databaseId);
  return path === null ? null : (itemFor(path)?.labelKey ?? null);
}

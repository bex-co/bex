import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import { filterNavGroups } from "@/common/components/dashboard-layout/dashboard-nav";
import {
  resourcePageNav,
  resourcePageNavItem,
} from "@/common/components/dashboard-layout/resource-page-nav";
import { ChartNoAxesCombined, Info, ScrollText } from "lucide-react";

// A Key Value instance's pages, as Render's sidebar lists them (w3/m649,
// docs/render-artifacts/key-value.md § Pages): Info, then **Monitor** (Logs,
// Metrics). Render's `/plan` is a sub-page with no entry; bex's plan picker
// is a card on Info, and `/plan` lands there.
//
// One source of truth: the Key Value sidebar
// (common/components/dashboard-layout/key-value-sidebar.tsx) draws these
// groups and the detail layout names each page with its entry's label.

/** A page's path under the instance, e.g. `/logs`. */
export type KeyValuePagePath = "/info" | "/logs" | "/metrics";

/** A page's route, typed for links and redirects. */
export function keyValuePageRoute<P extends KeyValuePagePath>(page: P) {
  return `/keyvalue/$keyValueId${page}` as const;
}

// Info is also the bare URL's page, as on Render (`hrefPattern: /r/<id>`).
const INFO = resourcePageNavItem(
  keyValuePageRoute("/info"),
  "keyvalue.navInfo",
  Info,
  [""],
);
const LOGS = resourcePageNavItem(
  keyValuePageRoute("/logs"),
  "keyvalue.navLogs",
  ScrollText,
);
const METRICS = resourcePageNavItem(
  keyValuePageRoute("/metrics"),
  "keyvalue.navMetrics",
  ChartNoAxesCombined,
);

const GROUPS: readonly SidebarNavGroup[] = [
  { items: [INFO] },
  { labelKey: "common.navMonitorGroup", items: [LOGS, METRICS] },
];

/**
 * The instance's nav groups. `logsDenied` is a confirmed can_view_logs denial
 * (a Viewer's or a Billing member's, w9/145) and drops Logs, as Render drops
 * it without `viewRedisLogs`; `metricsDenied` is a confirmed
 * can_view_activity denial (a Billing member's, w9/m621) and drops Metrics,
 * as Render drops it without `viewRedisMetrics`. A group left empty is
 * dropped. While capabilities are still checking both are false, so an
 * allowed member's nav never shifts.
 */
export function keyValueNavGroups({
  logsDenied = false,
  metricsDenied = false,
}: { logsDenied?: boolean; metricsDenied?: boolean } = {}): SidebarNavGroup[] {
  return filterNavGroups(
    [...GROUPS],
    (item) =>
      !(logsDenied && item === LOGS) && !(metricsDenied && item === METRICS),
  );
}

const NAV = resourcePageNav("/keyvalue/$keyValueId", GROUPS);

/** Whether the sidebar entry `to` is the page `pathname` is on. */
export function isKeyValueNavItemActive(
  pathname: string,
  keyValueId: string,
  to: string,
): boolean {
  return NAV.isActive(pathname, keyValueId, to);
}

/**
 * The open page's sidebar label, which is also its heading. Null off the
 * instance's pages.
 */
export function keyValuePageLabelKey(
  pathname: string,
  keyValueId: string,
): SidebarNavItem["labelKey"] | null {
  return NAV.labelKey(pathname, keyValueId);
}

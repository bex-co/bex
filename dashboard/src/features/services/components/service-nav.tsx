import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import { filterNavGroups } from "@/common/components/dashboard-layout/dashboard-nav";
import {
  Activity,
  ArrowRightLeft,
  Braces,
  ChartNoAxesCombined,
  Cpu,
  HardDrive,
  History,
  Rocket,
  ScrollText,
  Settings,
  SquareTerminal,
  Tags,
} from "lucide-react";
import type { ServiceTypeKey } from "@/features/services/types";
import type { ServiceBase } from "@/features/services/lib/service-base";

// bex's subset of Render's service-page navigation, grouped the way Render's
// resource-scoped sidebar groups it (live captures 2026-07-16 + 2026-07-18 +
// 2026-07-29, docs/render-artifacts/dashboard-routes.md § Sidebar navigation):
// top-level items, then **Monitor** and **Manage** — and type-gated the way
// Render gates it (w5/m48, w5/m57). A static_site has no runtime instance (it
// serves from the object store via the shared static-server,
// docs/ADR029-static-sites.md), so it gets no Logs, Shell, or Compute;
// instead its Manage group carries Render's two dedicated edge-rule pages,
// Redirects/Rewrites and Headers. It also mirrors Render's static IA (w5/m57):
// its primary entry is Events (promoted to a top-level item), not Deploys —
// Render's static sidebar has no Deploys tab (deploy history/detail is reached
// from the Events feed). A cron job leads with Runs, then Deploys, as Render's
// signed-in dashboard does (w3/m554, docs/render-artifacts/cron-dashboard.md;
// Render's docs still call its deploy list "Builds"). Every other type keeps
// Deploys as its primary entry because bex exposes dedicated deploy history
// and detail routes. Compute is Render's page for the compute plan and scaling
// (w6/m483, docs/render-artifacts/compute-page.md); a cron job has none, its
// plan being its Settings' Compute section. Shell leads to bex's
// running-instance SSH instructions. Disk shipped with ADR082 (the
// stateless-first non-goal was reversed 2026-08-22); Previews and One-Off Jobs
// remain DO_NOT_DO non-goals.
//
// One source of truth: the service sidebar
// (common/components/dashboard-layout/service-sidebar.tsx) renders these
// groups; nothing else may fork the list.

/** Build a service-nav item under `base` — its `to` stays within the current
 *  tree (`/services` or `/static`, Render parity w5/m57). `to` is a plain
 *  string on SidebarNavItem, so the base prefix needs no typed-route juggling. */
function navItem(
  base: ServiceBase,
  suffix: string,
  labelKey: SidebarNavItem["labelKey"],
  icon: SidebarNavItem["icon"],
): SidebarNavItem {
  return { labelKey, to: `${base}/$serviceId${suffix}`, icon };
}

/** The tabs of service activity — events (a cron job's runs among them),
 *  deploys and metrics — which Render's Billing role does not view. */
const ACTIVITY_TABS = new Set<SidebarNavItem["labelKey"]>([
  "services.navDeploys",
  "services.navRuns",
  "services.navEvents",
  "services.navMetrics",
]);

/**
 * The service's nav groups. `activityDenied` (w9/m621) is a confirmed
 * can_view_activity denial, a Billing member's, and drops the activity tabs;
 * `logsDenied` (w9/145) is a confirmed can_view_logs denial, a Viewer's or a
 * Billing member's, and drops Logs. Either drops any group it leaves empty.
 * While capabilities are still checking both are false, so an allowed
 * member's nav never shifts.
 */
export function serviceNavGroups(
  type: ServiceTypeKey | null,
  base: ServiceBase = "/services",
  {
    activityDenied = false,
    logsDenied = false,
  }: { activityDenied?: boolean; logsDenied?: boolean } = {},
): SidebarNavGroup[] {
  return filterNavGroups(
    serviceNavGroupsForType(type, base),
    (item) =>
      !(activityDenied && ACTIVITY_TABS.has(item.labelKey)) &&
      !(logsDenied && item.labelKey === "services.navLogs"),
  );
}

function serviceNavGroupsForType(
  type: ServiceTypeKey | null,
  base: ServiceBase,
): SidebarNavGroup[] {
  const DEPLOYS = navItem(base, "/deploys", "services.navDeploys", Rocket);
  // A cron job's run history, beside its deploys (w2/m360, w3/m554).
  const RUNS = navItem(base, "/runs", "services.navRuns", History);
  const SETTINGS = navItem(base, "/settings", "services.navSettings", Settings);
  const DISK = navItem(base, "/disk", "services.navDisk", HardDrive);
  const EVENTS = navItem(base, "/events", "services.navEvents", Activity);
  const LOGS = navItem(base, "/logs", "services.navLogs", ScrollText);
  const METRICS = navItem(
    base,
    "/metrics",
    "services.navMetrics",
    ChartNoAxesCombined,
  );
  const ENVIRONMENT = navItem(base, "/env", "services.navEnvironment", Braces);
  const SHELL = navItem(base, "/shell", "services.navShell", SquareTerminal);
  const COMPUTE = navItem(base, "/compute", "services.navCompute", Cpu);
  const REDIRECTS = navItem(
    base,
    "/redirects",
    "services.navRedirects",
    ArrowRightLeft,
  );
  const HEADERS = navItem(base, "/headers", "services.navHeaders", Tags);

  // A static_site matches Render's static IA (live capture, w5/m57): Events is
  // its top-level primary entry (no Deploys tab — deploy history/detail is
  // reached from the Events feed), it has no runtime Logs/Shell/Compute,
  // and its Manage group carries Render's two dedicated edge-rule pages.
  if (type === "static") {
    return [
      { items: [EVENTS, SETTINGS] },
      { labelKey: "common.navMonitorGroup", items: [METRICS] },
      {
        labelKey: "common.navManageGroup",
        items: [ENVIRONMENT, REDIRECTS, HEADERS],
      },
    ];
  }
  // While the type is still unknown (null), show only the entries that sit in
  // the same place for every type, so nothing type-specific flashes then
  // vanishes once it resolves — a static site must never flash a Deploys tab
  // or Shell/Compute, so those wait for the type. Events stays under
  // Monitor here (its non-static home); a static site repositions it to
  // top-level once its type is known.
  const topLevel =
    type === null
      ? [SETTINGS]
      : type === "cron"
        ? [RUNS, DEPLOYS, SETTINGS]
        : [DEPLOYS, SETTINGS];
  // Render orders Manage as Environment, Previews, Disk, One-Off Jobs, Shell;
  // bex has neither Previews nor Jobs, so Disk sits directly after Environment.
  //
  // Disk is type-gated to exactly what the API accepts — web, private, and
  // worker (validateDisk in lego/backend/internal/apps/disks.go). A cron job
  // has no long-running instance to mount a volume on, so offering the tab
  // there would lead to an Add Disk button that always 400s.
  const diskEligible =
    type === "web" || type === "private" || type === "worker";
  // Compute sits where Render's Manage group had Scaling. A cron job has no
  // Compute entry, as on Render: its plan is its Settings' Compute section.
  const manageTail =
    type === null
      ? []
      : [
          ...(diskEligible ? [DISK] : []),
          SHELL,
          ...(type === "cron" ? [] : [COMPUTE]),
        ];
  return [
    { items: topLevel },
    { labelKey: "common.navMonitorGroup", items: [EVENTS, LOGS, METRICS] },
    { labelKey: "common.navManageGroup", items: [ENVIRONMENT, ...manageTail] },
  ];
}

/**
 * The nav label of the tab `pathname` is exactly on — the tab's heading in
 * the detail layout, so every tab names itself with the same words as its
 * sidebar entry (on a phone the sidebar is in a drawer, and nothing else on
 * the page said which tab was open). Null below a tab (a deploy's detail
 * page carries its own title) or off the service tree.
 */
export function serviceTabLabelKey(
  pathname: string,
  serviceId: string,
  base: ServiceBase = "/services",
): SidebarNavItem["labelKey"] | null {
  const path = pathname.replace(/\/+$/, "");
  for (const type of ["web", "cron", "static"] as const) {
    for (const group of serviceNavGroups(type, base)) {
      for (const item of group.items) {
        if (item.to.replace("$serviceId", serviceId) === path) {
          return item.labelKey;
        }
      }
    }
  }
  return null;
}

/**
 * Whether `pathname` is a page Render draws without the service header (w3/106):
 * Logs, Metrics and Shell, and a deploy's detail. Render's service and cron
 * layouts pass `hideResourceHeader` there (`CronJob-*.js` `ya`, walked signed
 * in 2026-11-25), giving the log explorer, the charts, the terminal and the
 * deploy's log the page's full height. Every other page keeps the header.
 */
export function serviceHeaderHidden(
  pathname: string,
  serviceId: string,
  base: ServiceBase = "/services",
): boolean {
  const prefix = `${base}/${serviceId}/`;
  const path = pathname.replace(/\/+$/, "");
  if (!path.startsWith(prefix)) return false;
  return /^(?:logs|metrics|shell|deploys\/[^/]+)$/.test(
    path.slice(prefix.length),
  );
}

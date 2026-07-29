import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  Activity,
  ArrowRightLeft,
  Braces,
  ChartNoAxesCombined,
  CreditCard,
  Rocket,
  Scaling,
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
// docs/ADR029-static-sites.md), so it gets no Logs, Shell, Scaling, or Plan;
// instead its Manage group carries Render's two dedicated edge-rule pages,
// Redirects/Rewrites and Headers. It also mirrors Render's static IA (w5/m57):
// its primary entry is Events (promoted to a top-level item), not Deploys —
// Render's static sidebar has no Deploys tab (deploy history/detail is reached
// from the Events feed). Every other type keeps Deploys as its primary entry
// because bex exposes dedicated deploy history and detail routes. Plan is
// bex-only (Render folds instance type into scaling/settings). Shell leads to
// bex's running-instance SSH instructions. Previews, Disk, and One-Off Jobs
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

export function serviceNavGroups(
  type: ServiceTypeKey | null,
  base: ServiceBase = "/services",
): SidebarNavGroup[] {
  const DEPLOYS = navItem(base, "/deploys", "services.navDeploys", Rocket);
  const SETTINGS = navItem(base, "/settings", "services.navSettings", Settings);
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
  const SCALING = navItem(base, "/scaling", "services.navScaling", Scaling);
  const PLAN = navItem(base, "/plan", "services.navPlan", CreditCard);
  const REDIRECTS = navItem(
    base,
    "/redirects",
    "services.navRedirects",
    ArrowRightLeft,
  );
  const HEADERS = navItem(base, "/headers", "services.navHeaders", Tags);

  // A static_site matches Render's static IA (live capture, w5/m57): Events is
  // its top-level primary entry (no Deploys tab — deploy history/detail is
  // reached from the Events feed), it has no runtime Logs/Shell/Scaling/Plan,
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
  // or Shell/Scaling/Plan, so those wait for the type. Events stays under
  // Monitor here (its non-static home); a static site repositions it to
  // top-level once its type is known.
  const topLevel = type === null ? [SETTINGS] : [DEPLOYS, SETTINGS];
  const manageTail = type === null ? [] : [SHELL, SCALING, PLAN];
  return [
    { items: topLevel },
    { labelKey: "common.navMonitorGroup", items: [EVENTS, LOGS, METRICS] },
    { labelKey: "common.navManageGroup", items: [ENVIRONMENT, ...manageTail] },
  ];
}

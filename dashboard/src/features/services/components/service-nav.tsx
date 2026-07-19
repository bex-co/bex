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

// bex's subset of Render's service-page navigation, grouped the way Render's
// resource-scoped sidebar groups it (live captures 2026-07-16 + 2026-07-18,
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): top-level
// items, then **Monitor** and **Manage** — and type-gated the way Render gates
// it (w5/m48). A static_site has no runtime instance (it serves from the
// object store via the shared static-server, docs/ADR029-static-sites.md), so
// it gets no Logs, Shell, Scaling, or Plan; instead its Manage group carries
// Render's two dedicated edge-rule pages, Redirects/Rewrites and Headers.
// Deploys is the primary entry because bex exposes dedicated deploy history
// and detail routes. Plan is bex-only (Render folds instance type into
// scaling/settings). Shell leads to bex's running-instance SSH instructions.
// Previews, Disk, and One-Off Jobs remain DO_NOT_DO non-goals.
//
// One source of truth: the service sidebar
// (common/components/dashboard-layout/service-sidebar.tsx) renders these
// groups; nothing else may fork the list.

const DEPLOYS: SidebarNavItem = {
  labelKey: "services.navDeploys",
  to: "/services/$serviceId/deploys",
  icon: Rocket,
};
const SETTINGS: SidebarNavItem = {
  labelKey: "services.navSettings",
  to: "/services/$serviceId/settings",
  icon: Settings,
};
const EVENTS: SidebarNavItem = {
  labelKey: "services.navEvents",
  to: "/services/$serviceId/events",
  icon: Activity,
};
const LOGS: SidebarNavItem = {
  labelKey: "services.navLogs",
  to: "/services/$serviceId/logs",
  icon: ScrollText,
};
const METRICS: SidebarNavItem = {
  labelKey: "services.navMetrics",
  to: "/services/$serviceId/metrics",
  icon: ChartNoAxesCombined,
};
const ENVIRONMENT: SidebarNavItem = {
  labelKey: "services.navEnvironment",
  to: "/services/$serviceId/env",
  icon: Braces,
};
const SHELL: SidebarNavItem = {
  labelKey: "services.navShell",
  to: "/services/$serviceId/shell",
  icon: SquareTerminal,
};
const SCALING: SidebarNavItem = {
  labelKey: "services.navScaling",
  to: "/services/$serviceId/scaling",
  icon: Scaling,
};
const PLAN: SidebarNavItem = {
  labelKey: "services.navPlan",
  to: "/services/$serviceId/plan",
  icon: CreditCard,
};
const REDIRECTS: SidebarNavItem = {
  labelKey: "services.navRedirects",
  to: "/services/$serviceId/redirects",
  icon: ArrowRightLeft,
};
const HEADERS: SidebarNavItem = {
  labelKey: "services.navHeaders",
  to: "/services/$serviceId/headers",
  icon: Tags,
};

export function serviceNavGroups(
  type: ServiceTypeKey | null,
): SidebarNavGroup[] {
  const staticSite = type === "static";
  // A static_site has no pods: no runtime logs to tail (Render's static
  // sidebar has no Logs either — build logs live on the deploy detail page).
  const monitorItems = staticSite
    ? [EVENTS, METRICS]
    : [EVENTS, LOGS, METRICS];
  // While the type is still unknown (null), show only the entries every type
  // has — a static site must never flash Shell/Scaling/Plan, and a web
  // service gains them as soon as the type resolves.
  const manageTail = staticSite
    ? [REDIRECTS, HEADERS]
    : type === null
      ? []
      : [SHELL, SCALING, PLAN];
  return [
    { items: [DEPLOYS, SETTINGS] },
    { labelKey: "common.navMonitorGroup", items: monitorItems },
    { labelKey: "common.navManageGroup", items: [ENVIRONMENT, ...manageTail] },
  ];
}

import type { SidebarNavGroup } from "@/common/components/dashboard-layout/sidebar-nav-groups";

// bex's subset of Render's service-page navigation, grouped the way Render's
// resource-scoped sidebar groups it (live capture 2026-07-16,
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): top-level
// items, then **Monitor** (Logs, Metrics) and **Manage** (Environment,
// Scaling, Plan). Differences, both deliberate: Render has NO Deploys entry
// (its service root IS the deploy history — bex's root also redirects to
// Deploys, but the explicit entry aids discoverability), and Plan is bex-only
// (Render folds instance type into scaling/settings). Render's Shell,
// Previews, Disk, and One-Off Jobs entries are DO_NOT_DO non-goals.
//
// One source of truth: the service sidebar
// (common/components/dashboard-layout/service-sidebar.tsx) renders these
// groups; nothing else may fork the list.
export const SERVICE_NAV_GROUPS: SidebarNavGroup[] = [
  {
    items: [
      { labelKey: "services.navEvents", to: "/services/$serviceId/events" },
      { labelKey: "services.navDeploys", to: "/services/$serviceId/deploys" },
      { labelKey: "services.navSettings", to: "/services/$serviceId/settings" },
    ],
  },
  {
    labelKey: "common.navMonitorGroup",
    items: [
      { labelKey: "services.navLogs", to: "/services/$serviceId/logs" },
      { labelKey: "services.navMetrics", to: "/services/$serviceId/metrics" },
    ],
  },
  {
    labelKey: "common.navManageGroup",
    items: [
      { labelKey: "services.navEnvironment", to: "/services/$serviceId/env" },
      { labelKey: "services.navScaling", to: "/services/$serviceId/scaling" },
      { labelKey: "services.navPlan", to: "/services/$serviceId/plan" },
    ],
  },
];

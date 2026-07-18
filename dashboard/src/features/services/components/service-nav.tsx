import type { SidebarNavGroup } from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  Activity,
  Braces,
  ChartNoAxesCombined,
  CreditCard,
  Scaling,
  ScrollText,
  Settings,
  SquareTerminal,
} from "lucide-react";

// bex's subset of Render's service-page navigation, grouped the way Render's
// resource-scoped sidebar groups it (live capture 2026-07-16,
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): top-level
// items, then **Monitor** (Logs, Metrics) and **Manage** (Environment,
// Scaling, Plan). Differences, both deliberate: Render has NO Deploys entry
// (its service root IS the deploy history — bex's unified Events page shows
// both deploys and audit events, matching Render's behavior; w1/m47). Plan is
// bex-only (Render folds instance type into scaling/settings). Shell leads to
// bex's running-instance SSH instructions; it does not embed Render's
// browser-hosted terminal. Previews, Disk, and One-Off Jobs remain DO_NOT_DO
// non-goals.
//
// One source of truth: the service sidebar
// (common/components/dashboard-layout/service-sidebar.tsx) renders these
// groups; nothing else may fork the list.
export const SERVICE_NAV_GROUPS: SidebarNavGroup[] = [
  {
    items: [
      {
        labelKey: "services.navEvents",
        to: "/services/$serviceId/events",
        icon: Activity,
      },
      {
        labelKey: "services.navSettings",
        to: "/services/$serviceId/settings",
        icon: Settings,
      },
    ],
  },
  {
    labelKey: "common.navMonitorGroup",
    items: [
      {
        labelKey: "services.navLogs",
        to: "/services/$serviceId/logs",
        icon: ScrollText,
      },
      {
        labelKey: "services.navMetrics",
        to: "/services/$serviceId/metrics",
        icon: ChartNoAxesCombined,
      },
    ],
  },
  {
    labelKey: "common.navManageGroup",
    items: [
      {
        labelKey: "services.navEnvironment",
        to: "/services/$serviceId/env",
        icon: Braces,
      },
      {
        labelKey: "services.navShell",
        to: "/services/$serviceId/shell",
        icon: SquareTerminal,
      },
      {
        labelKey: "services.navScaling",
        to: "/services/$serviceId/scaling",
        icon: Scaling,
      },
      {
        labelKey: "services.navPlan",
        to: "/services/$serviceId/plan",
        icon: CreditCard,
      },
    ],
  },
];

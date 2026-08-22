import { lazy, Suspense, useCallback } from "react";
import { useParams, useRouterState } from "@tanstack/react-router";
import {
  Bell,
  Bot,
  Boxes,
  CreditCard,
  FolderKanban,
  Layers,
  Settings,
  Webhook,
} from "lucide-react";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from "@/common/components/ui/sidebar.tsx";
import { isNavItemActive } from "./nav-active";
import { SidebarBrand } from "./sidebar-brand";
import { SidebarNavGroups, type SidebarNavGroup } from "./sidebar-nav-groups";

// Contextual rails are route-scoped; keep them out of the always-mounted chrome
// chunk until the matching pathname/param is active (vercel bundle-dynamic /
// bundle-conditional).
const AgentSessionsNavSection = lazy(() =>
  import("./agent-sessions-nav-section").then((m) => ({
    default: m.AgentSessionsNavSection,
  })),
);
const ProjectSidebar = lazy(() =>
  import("./project-sidebar").then((m) => ({ default: m.ProjectSidebar })),
);
const ServiceSidebar = lazy(() =>
  import("./service-sidebar").then((m) => ({ default: m.ServiceSidebar })),
);

// Render parity: one "Projects" entry groups every resource type (services,
// databases, key value) on a single page (`routes/index.tsx`), rather than a
// separate nav item per resource kind — `/databases`/`/keyvalue` still exist as
// routes (detail pages, deep links) but no longer get their own sidebar entry.
// The sidebar sits under the workspace switcher, so every entry in it is
// workspace-scoped — Settings included, pointing at the workspace's own settings
// (team, plan, integrations). Account settings (`/settings`) belong to the
// user, not the workspace, and hang off the header's user menu instead —
// Render's own split.
//
// Grouping mirrors Render's live sidebar (2026-07-16 capture,
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): an
// unlabeled top group (Projects/Blueprints/Environment Groups), then
// **Integrations** (Webhooks, Notifications — Render also lists
// Observability, a drains non-goal) and **Workspace** (Usage standing in for
// Render's Billing — ADR023 — plus Settings). Render's Networking group
// (Private Links, Dedicated IPs) is omitted entirely: both entries are
// DO_NOT_DO non-goals, and an empty group is worse than none.
const NAV_GROUPS: SidebarNavGroup[] = [
  {
    items: [
      { labelKey: "common.navProjects", to: "/", icon: FolderKanban },
      { labelKey: "common.navBlueprints", to: "/blueprints", icon: Layers },
      { labelKey: "common.navAgents", to: "/agents", icon: Bot },
      { labelKey: "common.navEnvGroups", to: "/env-groups", icon: Boxes },
    ],
  },
  {
    labelKey: "common.navIntegrationsGroup",
    items: [
      { labelKey: "common.navWebhooks", to: "/webhooks", icon: Webhook },
      { labelKey: "common.navNotifications", to: "/notifications", icon: Bell },
    ],
  },
  {
    labelKey: "common.navWorkspaceGroup",
    items: [
      { labelKey: "common.navUsage", to: "/billing", icon: CreditCard },
      {
        labelKey: "common.navSettings",
        to: "/workspace/settings",
        icon: Settings,
      },
    ],
  },
];

export function DashboardSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { projectId, serviceId } = useParams({ strict: false });
  const isItemActive = useCallback(
    (to: string) => isNavItemActive(pathname, to),
    [pathname],
  );

  if (projectId) {
    return (
      <Suspense fallback={<SidebarShell />}>
        <ProjectSidebar projectId={projectId} />
      </Suspense>
    );
  }
  if (serviceId) {
    return (
      <Suspense fallback={<SidebarShell />}>
        <ServiceSidebar serviceId={serviceId} />
      </Suspense>
    );
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent className="gap-0">
        <SidebarNavGroups groups={NAV_GROUPS} isItemActive={isItemActive} />
        {/* The contextual list slot (w5/m64). Unlike ProjectSidebar and
            ServiceSidebar above — which REPLACE the rail for a deep hierarchy
            and offer a back link — an agents-context section AUGMENTS the nav,
            Devin's own shape: global nav on top, the section's working set
            beneath. Section-scoped on purpose: sessions never follow you onto
            Projects/Services/Settings. See ADR047 D9. */}
        {isNavItemActive(pathname, "/agents") ? (
          <Suspense fallback={null}>
            <AgentSessionsNavSection />
          </Suspense>
        ) : null}
      </SidebarContent>
    </Sidebar>
  );
}

/** Minimal chrome while a contextual rail chunk loads. */
function SidebarShell() {
  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent className="gap-0" />
    </Sidebar>
  );
}

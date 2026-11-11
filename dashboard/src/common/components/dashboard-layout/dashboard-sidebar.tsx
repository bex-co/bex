import { lazy, Suspense, useCallback, useMemo } from "react";
import { useParams, useRouterState } from "@tanstack/react-router";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from "@/common/components/ui/sidebar.tsx";
import { useAgentsFeatureEnabled } from "@/config/use-growthbook";
import { isNavItemActive } from "./nav-active";
import { SidebarBrand } from "./sidebar-brand";
import { SidebarNavGroups } from "./sidebar-nav-groups";
import { NAV_GROUPS, visibleNavGroups } from "./dashboard-nav";
import { useRouterNavAvailable } from "./use-router-nav-available";

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

export function DashboardSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { projectId, serviceId } = useParams({ strict: false });
  const agentsEnabled = useAgentsFeatureEnabled();
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
        <TopLevelNavGroups
          agentsEnabled={agentsEnabled}
          isItemActive={isItemActive}
        />
        {/* The contextual list slot (w5/m64). Unlike ProjectSidebar and
            ServiceSidebar above — which REPLACE the rail for a deep hierarchy
            and offer a back link — an agents-context section AUGMENTS the nav,
            Devin's own shape: global nav on top, the section's working set
            beneath. Section-scoped on purpose: sessions never follow you onto
            Projects/Services/Settings. See ADR047 D9. */}
        {agentsEnabled && isNavItemActive(pathname, "/agents") ? (
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

/**
 * The top-level destinations. It sits below the project and service rails so
 * only the rail that shows them asks whether the workspace is offered Router.
 */
function TopLevelNavGroups({
  agentsEnabled,
  isItemActive,
}: {
  agentsEnabled: boolean;
  isItemActive: (to: string) => boolean;
}) {
  const routerAvailable = useRouterNavAvailable();
  const groups = useMemo(
    () => visibleNavGroups(NAV_GROUPS, { agentsEnabled, routerAvailable }),
    [agentsEnabled, routerAvailable],
  );
  return <SidebarNavGroups groups={groups} isItemActive={isItemActive} />;
}

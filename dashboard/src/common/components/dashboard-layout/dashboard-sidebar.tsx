import { useQuery } from "@apollo/client/react";
import { RouterAvailableDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context";
import { lazy, Suspense, useCallback, useMemo } from "react";
import { useParams, useRouterState } from "@tanstack/react-router";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from "@/common/components/ui/sidebar.tsx";
import {
  useAgentsFeatureEnabled,
  useRouterFeatureEnabled,
} from "@/config/use-growthbook";
import { isNavItemActive } from "./nav-active";
import { SidebarBrand } from "./sidebar-brand";
import { SidebarNavGroups, type SidebarNavGroup } from "./sidebar-nav-groups";
import { NAV_GROUPS, visibleNavGroups } from "./dashboard-nav";

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
  const routerEnabled = useRouterFeatureEnabled();
  const navGroups = useMemo(
    () => visibleNavGroups(NAV_GROUPS, { agentsEnabled, routerEnabled }),
    [agentsEnabled, routerEnabled],
  );
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
        {routerEnabled ? (
          <RouterNavGroups groups={navGroups} isItemActive={isItemActive} />
        ) : (
          <SidebarNavGroups groups={navGroups} isItemActive={isItemActive} />
        )}
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

/** Only targeted workspaces probe the optional companion service. */
function RouterNavGroups({
  groups,
  isItemActive,
}: {
  groups: SidebarNavGroup[];
  isItemActive: (to: string) => boolean;
}) {
  const { currentWorkspaceId } = useWorkspace();
  const { data } = useQuery(RouterAvailableDocument, {
    variables: { ownerId: currentWorkspaceId ?? "" },
  });
  const available = data?.routerAvailable === true;
  return (
    <SidebarNavGroups
      groups={
        available
          ? groups
          : groups.map((group) => ({
              ...group,
              items: group.items.filter((item) => item.to !== "/router"),
            }))
      }
      isItemActive={isItemActive}
    />
  );
}

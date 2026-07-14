import { Link, useParams, useRouterState } from "@tanstack/react-router";
import { BarChart3, Boxes, FolderKanban, Layers, Settings } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/common/components/ui/sidebar.tsx";
import { isNavItemActive } from "./nav-active";
import { ProjectSidebar } from "./project-sidebar";
import { SidebarBrand } from "./sidebar-brand";

// Render parity: one "Projects" entry groups every resource type (services,
// databases, key value) on a single page (`routes/index.tsx`), rather than a
// separate nav item per resource kind — `/databases`/`/keyvalue` still exist as
// routes (detail pages, deep links) but no longer get their own sidebar entry.
// The sidebar sits under the workspace switcher, so every entry in it is
// workspace-scoped — Settings included, pointing at the workspace's own settings
// (team, plan, API keys, integrations, audit). Account settings (`/settings`)
// belong to the user, not the workspace, and hang off the header's user menu
// instead — Render's own split.
const NAV_ITEMS = [
  { labelKey: "common.navProjects", to: "/", icon: FolderKanban },
  { labelKey: "common.navBlueprints", to: "/blueprints", icon: Layers },
  { labelKey: "common.navEnvGroups", to: "/env-groups", icon: Boxes },
  { labelKey: "common.navUsage", to: "/usage", icon: BarChart3 },
  { labelKey: "common.navSettings", to: "/workspace/settings", icon: Settings },
] as const;

export function DashboardSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { t } = useTranslations();
  const { projectId } = useParams({ strict: false });

  if (projectId) {
    return <ProjectSidebar projectId={projectId} />;
  }

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{t("common.navDashboardGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV_ITEMS.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton
                    asChild
                    isActive={isNavItemActive(pathname, item.to)}
                  >
                    <Link to={item.to}>
                      <item.icon />
                      <span>{t(item.labelKey)}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}

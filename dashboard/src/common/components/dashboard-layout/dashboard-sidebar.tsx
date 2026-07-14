import { Link, useRouterState } from "@tanstack/react-router";
import { BarChart3, FolderKanban, Settings } from "lucide-react";
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
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";
import { isNavItemActive } from "./nav-active";

// Render parity: one "Projects" entry groups every resource type (services,
// databases, key value) on a single page (`routes/index.tsx`), rather than a
// separate nav item per resource kind — `/databases`/`/keyvalue` still exist as
// routes (detail pages, deep links) but no longer get their own sidebar entry.
const NAV_ITEMS = [
  { labelKey: "common.navProjects", to: "/", icon: FolderKanban },
  { labelKey: "common.navUsage", to: "/usage", icon: BarChart3 },
  { labelKey: "common.navSettings", to: "/settings", icon: Settings },
] as const;

export function DashboardSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { t } = useTranslations();

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <WorkspaceSwitcher />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{t("common.navDashboardGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV_ITEMS.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton asChild isActive={isNavItemActive(pathname, item.to)}>
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

import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronLeft, FolderKanban, Settings } from "lucide-react";
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
import { useProjects } from "@/features/projects/hooks/use-projects";
import { SidebarBrand } from "./sidebar-brand";

export interface ProjectSidebarProps {
  projectId: string;
}

/**
 * Render parity: a project's own pages (`/project/:id`, `/project/:id/settings`)
 * swap the global workspace nav for a contextual one scoped to that project —
 * a back link one level up, the project name, "Overview", and a "Manage"
 * group holding "Settings". The logo + workspace switcher (`SidebarBrand`)
 * stay pinned at the top exactly as on every other page — only the nav below
 * it is contextual. The back link's destination and label follow Render's
 * own behavior: from the settings sub-page it goes back up to the project's
 * Overview ("Project"); from Overview it goes back up to the workspace
 * Overview ("Dashboard").
 */
export function ProjectSidebar({ projectId }: ProjectSidebarProps) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { t } = useTranslations();
  const { projects } = useProjects();
  const projectName =
    projects.find((p) => p.id === projectId)?.name ?? projectId;
  const settingsPath = `/project/${projectId}/settings`;
  const onSettings =
    pathname === settingsPath || pathname.startsWith(`${settingsPath}/`);

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent>
        <div className="flex flex-col gap-2 px-2 pt-2 pb-1">
          {onSettings ? (
            <Link
              to="/project/$projectId"
              params={{ projectId }}
              className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
            >
              <ChevronLeft className="size-3.5" />
              {t("common.navBackToProject")}
            </Link>
          ) : (
            <Link
              to="/"
              className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
            >
              <ChevronLeft className="size-3.5" />
              {t("common.navBackToDashboard")}
            </Link>
          )}
          <p className="truncate text-sm font-semibold">{projectName}</p>
        </div>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={!onSettings}>
                  <Link to="/project/$projectId" params={{ projectId }}>
                    <FolderKanban />
                    <span>{t("common.navProjectOverview")}</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        <SidebarGroup>
          <SidebarGroupLabel>{t("common.navManageGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={onSettings}>
                  <Link
                    to="/project/$projectId/settings"
                    params={{ projectId }}
                  >
                    <Settings />
                    <span>{t("common.navSettings")}</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}

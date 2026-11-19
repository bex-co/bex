import { useState, type ComponentProps } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  ChevronLeft,
  FolderKanban,
  Globe,
  Plus,
  ScrollText,
  Settings,
} from "lucide-react";
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
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  EditViewDialog,
  type EditViewTarget,
} from "@/features/logs/components/edit-view-dialog";
import { useOpenView } from "@/features/logs/hooks/use-open-view";
import {
  PROJECT_LOG_VIEW_TITLE_KEYS,
  PROJECT_LOG_VIEW_TYPES,
  isProjectLogView,
  type ProjectLogViewType,
} from "@/features/logs/lib/project-log-views";
import { freeViewName } from "@/features/logs/lib/view-filters";
import { useProjects } from "@/features/projects/hooks/use-projects";
import {
  useSavedSearches,
  type SavedSearch,
} from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";
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
 *
 * Between Overview and Manage sits Render's OBSERVABILITY group (w3/m505):
 * one link per log view the project keeps (its saved searches of type
 * Application Logs and Request Logs). The Service Metrics view exists but is
 * not listed, as Render does not list it. Below them Render's **+ New View**
 * opens its TYPE menu (w3/m507): picking a type opens the Edit view modal
 * with that type's name, and the view is created on Done, then opened.
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
  const onObservability = pathname.startsWith(
    `/project/${projectId}/observability/`,
  );
  const savedSearchId = useRouterState({
    select: (s): string | undefined => {
      const id = (s.location.search as Record<string, unknown>).savedSearchId;
      return typeof id === "string" ? id : undefined;
    },
  });
  const { views, loading: viewsLoading } = useSavedSearches(projectId);
  const logViews = views.filter(isProjectLogView);

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent>
        <div className="flex flex-col gap-2 px-2 pt-2 pb-1 group-data-[collapsible=icon]:hidden">
          {onSettings || onObservability ? (
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
                <SidebarMenuButton
                  asChild
                  isActive={!onSettings && !onObservability}
                  tooltip={t("common.navProjectOverview")}
                >
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
          <SidebarGroupLabel>
            {t("common.navObservabilityGroup")}
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {viewsLoading
                ? // Render's two default views, reserved while the list loads
                  // (fixed widths: the menu skeleton's random one differs
                  // between the server and the client).
                  ["w-28", "w-24"].map((width) => (
                    <SidebarMenuItem key={width}>
                      <div
                        data-sidebar="menu-skeleton"
                        className="flex h-8 items-center gap-2 rounded-md px-2"
                      >
                        <Skeleton className="size-4 rounded-md" />
                        <Skeleton className={`h-4 ${width}`} />
                      </div>
                    </SidebarMenuItem>
                  ))
                : logViews.map((view) => (
                    <SidebarMenuItem key={view.id}>
                      <SidebarMenuButton
                        asChild
                        isActive={onObservability && savedSearchId === view.id}
                        tooltip={view.name}
                      >
                        <LogViewLink projectId={projectId} view={view} />
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
              <NewViewItem
                projectId={projectId}
                takenNames={views.map((view) => view.name)}
              />
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        <SidebarGroup>
          <SidebarGroupLabel>{t("common.navManageGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  isActive={onSettings}
                  tooltip={t("common.navSettings")}
                >
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

/**
 * Render's **+ New View** and its TYPE menu (w3/m507). A member who may not
 * edit the project sees it disabled with the reason, as every role-gated
 * control does; bex-api refuses the create too.
 */
function NewViewItem({
  projectId,
  takenNames,
}: {
  projectId: string;
  takenNames: string[];
}) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const openView = useOpenView(projectId);
  const [target, setTarget] = useState<EditViewTarget | null>(null);
  const reasonKey = capabilities.reasonKey("can_create");

  if (!capabilities.canCreate) {
    return (
      <SidebarMenuItem>
        <PermissionTooltip
          reason={reasonKey ? t(reasonKey) : undefined}
          triggerAsChild
        >
          <SidebarMenuButton
            aria-disabled="true"
            className="text-muted-foreground aria-disabled:cursor-not-allowed"
          >
            <Plus />
            <span>{t("logs.newView")}</span>
          </SidebarMenuButton>
        </PermissionTooltip>
      </SidebarMenuItem>
    );
  }
  return (
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <SidebarMenuButton
            tooltip={t("logs.newView")}
            className="text-muted-foreground"
          >
            <Plus />
            <span>{t("logs.newView")}</span>
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="right" align="start" className="w-48">
          <DropdownMenuLabel className="text-xs tracking-wider text-muted-foreground uppercase">
            {t("logs.newViewTypeHeading")}
          </DropdownMenuLabel>
          {PROJECT_LOG_VIEW_TYPES.map((type) => (
            <DropdownMenuItem
              key={type}
              onSelect={() =>
                setTarget({
                  kind: "create",
                  type,
                  name: freeViewName(
                    t(PROJECT_LOG_VIEW_TITLE_KEYS[type]),
                    takenNames,
                  ),
                })
              }
            >
              <ViewTypeIcon type={type} />
              {t(PROJECT_LOG_VIEW_TITLE_KEYS[type])}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <EditViewDialog
        projectId={projectId}
        target={target}
        onOpenChange={(open) => {
          if (!open) setTarget(null);
        }}
        onSaved={(view) => void openView(view)}
      />
    </SidebarMenuItem>
  );
}

function ViewTypeIcon({ type }: { type: ProjectLogViewType }) {
  return type === SavedSearchType.RequestLogs ? <Globe /> : <ScrollText />;
}

// One view's sidebar link, Render's: the view page by `savedSearchId`. The
// menu button's Slot hands its class and state attributes through `props`.
function LogViewLink({
  projectId,
  view,
  ...props
}: {
  projectId: string;
  view: SavedSearch & { type: ProjectLogViewType };
} & Omit<ComponentProps<"a">, "href" | "children">) {
  const content = (
    <>
      <ViewTypeIcon type={view.type} />
      <span>{view.name}</span>
    </>
  );
  return view.type === SavedSearchType.RequestLogs ? (
    <Link
      {...props}
      to="/project/$projectId/observability/logs/request"
      params={{ projectId }}
      search={{ savedSearchId: view.id }}
    >
      {content}
    </Link>
  ) : (
    <Link
      {...props}
      to="/project/$projectId/observability/logs/application"
      params={{ projectId }}
      search={{ savedSearchId: view.id }}
    >
      {content}
    </Link>
  );
}

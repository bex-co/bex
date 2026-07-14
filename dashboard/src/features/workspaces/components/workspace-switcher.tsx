import { useNavigate } from "@tanstack/react-router";
import { Building2, Check, ChevronsUpDown, Plus, Settings } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/common/components/ui/sidebar.tsx";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * The dropdown at the top of the left pane (Render's own placement, captured
 * in w6/RESEARCH-workspaces.md finding 2-3): current workspace, the caller's
 * full list to switch between, then Workspace Settings and + New Workspace.
 * Switching writes through WorkspaceProvider, which every scoped query
 * (services/databases) reads as its ownerId filter.
 */
export function WorkspaceSwitcher() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { workspaces, currentWorkspace, setCurrentWorkspaceId, loading } =
    useWorkspace();

  if (loading && workspaces.length === 0) {
    return (
      <div className="flex items-center gap-2 px-2 py-1.5">
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground">
              <Building2 className="size-4 shrink-0" />
              <span className="truncate font-medium">
                {currentWorkspace?.name ?? t("workspaces.switcherEmpty")}
              </span>
              <ChevronsUpDown className="ml-auto size-4 opacity-50" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
            align="start"
          >
            <DropdownMenuLabel className="text-muted-foreground text-xs">
              {t("workspaces.switcherLabel")}
            </DropdownMenuLabel>
            {workspaces.map((w) => (
              <DropdownMenuItem
                key={w.id}
                onSelect={() => setCurrentWorkspaceId(w.id)}
              >
                <span className="flex-1 truncate">{w.name}</span>
                {w.id === currentWorkspace?.id && <Check className="size-4" />}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => void navigate({ to: "/workspace/settings" })}
            >
              <Settings className="size-4" />
              {t("workspaces.switcherSettings")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => void navigate({ to: "/new/workspace" })}
            >
              <Plus className="size-4" />
              {t("workspaces.switcherNew")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}

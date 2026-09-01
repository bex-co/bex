import { useNavigate } from "@tanstack/react-router";
import {
  Check,
  ChevronsUpDown,
  CreditCard,
  Plus,
  Settings,
} from "lucide-react";
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
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { workspacePlanNameKey } from "@/features/workspaces/types";

/**
 * A small rounded tile showing the workspace's first letter — the switcher's
 * per-workspace identity mark. It replaces a generic building glyph (which was
 * identical for every workspace) so the trigger and the list rows are scannable
 * at a glance, and it's the one visual the trigger and dropdown rows share.
 */
function WorkspaceInitial({
  name,
  className,
}: {
  name: string;
  className?: string;
}) {
  const initial = name.trim().charAt(0).toUpperCase() || "?";
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex aspect-square items-center justify-center rounded-md bg-primary/10 text-[0.7rem] font-semibold text-primary",
        className,
      )}
    >
      {initial}
    </span>
  );
}

/**
 * The dropdown at the top of the left pane (Render's own placement): Billing,
 * Workspace Settings, the caller's full list (name + plan), then + New Workspace.
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
              <WorkspaceInitial
                name={currentWorkspace?.name ?? "?"}
                className="size-5 shrink-0"
              />
              <span className="truncate font-medium">
                {currentWorkspace?.name ?? t("workspaces.switcherEmpty")}
              </span>
              <ChevronsUpDown className="ml-auto size-4 shrink-0 opacity-50" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
            align="start"
          >
            <DropdownMenuItem
              onSelect={() => void navigate({ to: "/billing" })}
            >
              <CreditCard className="size-4" />
              {t("workspaces.switcherBilling")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => void navigate({ to: "/workspace/settings" })}
            >
              <Settings className="size-4" />
              {t("workspaces.switcherSettings")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-muted-foreground text-xs">
              {t("workspaces.switcherLabel")}
            </DropdownMenuLabel>
            {workspaces.map((w) => {
              const planKey = workspacePlanNameKey(w.plan);
              return (
                <DropdownMenuItem
                  key={w.id}
                  onSelect={() => setCurrentWorkspaceId(w.id)}
                  className="gap-2"
                >
                  <WorkspaceInitial name={w.name} className="size-5 shrink-0" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate">{w.name}</span>
                    {planKey ? (
                      <span className="text-muted-foreground truncate text-xs">
                        {t(planKey)}
                      </span>
                    ) : (
                      <span className="text-muted-foreground truncate text-xs">
                        {w.plan}
                      </span>
                    )}
                  </span>
                  {w.id === currentWorkspace?.id && (
                    <Check className="size-4 shrink-0" />
                  )}
                </DropdownMenuItem>
              );
            })}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() =>
                void navigate({
                  to: "/new/workspace",
                  search: { attempt: undefined },
                })
              }
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

import { useMemo } from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import {
  Building2,
  Check,
  ChevronsUpDown,
  CreditCard,
  Plus,
  Settings,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
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
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  workspacePlanNameKey,
  type WorkspaceView,
} from "@/features/workspaces/types";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import { groupSwitcherWorkspaces } from "@/features/organizations/lib/switcher-groups";
import { WorkspaceAvatar } from "@/features/avatars/components/avatar";

/**
 * The workspace's avatar, the switcher's per-workspace identity mark, as on
 * Render (w1/m648): its uploaded image, else the monogram of its name colored
 * by its id. The trigger and the list rows share it.
 */
function WorkspaceMark({
  workspace,
}: {
  workspace: Pick<WorkspaceView, "id" | "name" | "avatarURL"> | null;
}) {
  if (!workspace) {
    return (
      <span
        aria-hidden="true"
        className="size-5 shrink-0 rounded-full bg-muted"
      />
    );
  }
  return (
    <WorkspaceAvatar
      id={workspace.id}
      name={workspace.name}
      imageURL={workspace.avatarURL}
      size="small"
    />
  );
}

/** One workspace of the dropdown: its avatar, name and plan, checked when current. */
function WorkspaceRadioItem({
  workspace,
  current,
  nested = false,
  onSelect,
}: {
  workspace: WorkspaceView;
  current: boolean;
  /** Listed under its organization's entry. */
  nested?: boolean;
  onSelect: () => void;
}) {
  const { t } = useTranslations();
  const planKey = workspacePlanNameKey(workspace.plan);
  return (
    <DropdownMenuRadioItem
      value={workspace.id}
      onSelect={onSelect}
      // The radio dot is replaced by the visible check below (w4/471).
      className={cn(
        "gap-2 [&>span:first-child]:hidden",
        nested ? "pl-5" : "pl-2",
      )}
    >
      <WorkspaceMark workspace={workspace} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate">{workspace.name}</span>
        <span className="text-muted-foreground truncate text-xs">
          {planKey ? t(planKey) : workspace.plan}
        </span>
      </span>
      {current && <Check className="size-4 shrink-0" />}
    </DropdownMenuRadioItem>
  );
}

/**
 * The dropdown at the top of the left pane (Render's own placement): Billing,
 * Workspace Settings, the caller's list (name + plan), then + New Workspace.
 * Switching writes through WorkspaceProvider, which every scoped query
 * (services/databases) reads as its ownerId filter.
 *
 * Organizations (w7/m464) sit in it as on Render ("open the workspace dropdown
 * at the top of the left pane and select your org"): each org's name opens
 * its Workspaces page, with the caller's workspaces in it listed beneath; the
 * workspaces in no org follow under the usual label. A Guest belongs to one
 * workspace, so their org shows only that one.
 */
export function WorkspaceSwitcher() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  // Any path param names one resource (project, service, database, …) of the
  // workspace being left; staying on it after a switch shows that workspace's
  // data under the new one's chrome. Workspace-level pages (billing, settings,
  // the overview) re-render for the new workspace in place.
  const onResourceRoute = Object.keys(useParams({ strict: false })).length > 0;
  const { workspaces, currentWorkspace, setCurrentWorkspaceId, loading } =
    useWorkspace();
  const { organizations } = useOrganizations();
  // Billing details are Admin, Developer and Billing (w9/m620), as in the
  // sidebar: hidden once the role is confirmed not to hold them.
  const billingDenied = useCapabilities().denied("can_view_billing");
  const groups = useMemo(
    () => groupSwitcherWorkspaces(workspaces, organizations),
    [workspaces, organizations],
  );

  const switchTo = (id: string) => {
    if (id === currentWorkspace?.id) return;
    setCurrentWorkspaceId(id);
    if (onResourceRoute) void navigate({ to: "/" });
  };
  const radioItem = (workspace: WorkspaceView, nested?: boolean) => (
    <WorkspaceRadioItem
      key={workspace.id}
      workspace={workspace}
      current={workspace.id === currentWorkspace?.id}
      nested={nested}
      onSelect={() => switchTo(workspace.id)}
    />
  );

  if (loading && workspaces.length === 0) {
    // The loaded trigger's exact box (SidebarMenuButton: h-8, p-2, gap-2) with
    // its badge, name and chevron, so the 48px header and every nav link
    // below it stay put when the workspaces arrive (w4/440).
    return (
      <SidebarMenu>
        <SidebarMenuItem>
          <div
            data-slot="workspace-switcher-pending"
            aria-hidden="true"
            className="flex h-8 w-full items-center gap-2 overflow-hidden rounded-md p-2 group-data-[collapsible=icon]:size-8!"
          >
            <Skeleton className="size-5 shrink-0" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="ml-auto size-4 shrink-0" />
          </div>
        </SidebarMenuItem>
      </SidebarMenu>
    );
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground">
              <WorkspaceMark workspace={currentWorkspace ?? null} />
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
            {billingDenied ? null : (
              <DropdownMenuItem
                onSelect={() => void navigate({ to: "/billing" })}
              >
                <CreditCard className="size-4" />
                {t("workspaces.switcherBilling")}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              onSelect={() => void navigate({ to: "/workspace/settings" })}
            >
              <Settings className="size-4" />
              {t("workspaces.switcherSettings")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {groups.organizations.map((organization) => (
              <DropdownMenuGroup key={organization.id}>
                <DropdownMenuItem
                  onSelect={() =>
                    void navigate({
                      to: "/org/$orgId/workspaces",
                      params: { orgId: organization.id },
                    })
                  }
                  className="gap-2 pl-2"
                >
                  <span
                    aria-hidden="true"
                    className="flex size-5 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground"
                  >
                    <Building2 className="size-3.5" />
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate font-medium">
                      {organization.name}
                    </span>
                    <span className="text-muted-foreground truncate text-xs">
                      {t("organizations.switcherOrganization")}
                    </span>
                  </span>
                </DropdownMenuItem>
                {organization.workspaces.length > 0 ? (
                  <DropdownMenuRadioGroup value={currentWorkspace?.id ?? ""}>
                    {organization.workspaces.map((w) => radioItem(w, true))}
                  </DropdownMenuRadioGroup>
                ) : null}
                <DropdownMenuSeparator />
              </DropdownMenuGroup>
            ))}
            {groups.standalone.length > 0 ? (
              <>
                <DropdownMenuLabel className="text-muted-foreground text-xs">
                  {t("workspaces.switcherLabel")}
                </DropdownMenuLabel>
                <DropdownMenuRadioGroup value={currentWorkspace?.id ?? ""}>
                  {groups.standalone.map((w) => radioItem(w))}
                </DropdownMenuRadioGroup>
                <DropdownMenuSeparator />
              </>
            ) : null}
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

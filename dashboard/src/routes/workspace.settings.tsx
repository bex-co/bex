import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import { DeleteWorkspaceCard } from "@/features/workspaces/components/delete-workspace-card";

export const Route = createFileRoute("/workspace/settings")({
  component: WorkspaceSettingsPage,
  beforeLoad: requireAuth("/workspace/settings"),
  head: () => ({
    meta: [{ title: "Workspace Settings · bex dashboard" }],
  }),
});

/**
 * Workspace settings (w6/m3/t003-t004): the currently selected workspace's
 * rename/plan/metadata card plus its delete danger zone — distinct from
 * `/settings` (account settings, Kratos-backed) since a workspace and a user
 * account are different objects with different owners.
 */
export function WorkspaceSettingsPage() {
  const { t } = useTranslations();
  const { currentWorkspace, loading } = useWorkspace();

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          {!currentWorkspace && loading ? (
            <Skeleton className="h-64 w-full" />
          ) : !currentWorkspace ? (
            <p className="text-muted-foreground text-sm">
              {t("workspaces.settingsEmpty")}
            </p>
          ) : (
            <>
              <WorkspaceDetailsCard workspace={currentWorkspace} />
              <DeleteWorkspaceCard workspace={currentWorkspace} />
            </>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

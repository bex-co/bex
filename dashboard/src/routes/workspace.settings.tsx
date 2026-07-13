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
  // `?plan=change` arrives from the blocked-invite CTA (w6/m15/t001) and opens
  // the change-plan dialog straight away, so an invite the plan refused is one
  // click from the upgrade that would allow it. Advisory: anything else just
  // renders the settings page.
  // Optional, not `plan: undefined` — an always-present key would make `search`
  // mandatory on every navigation to this route (the switcher's included).
  validateSearch: (search: Record<string, unknown>): { plan?: "change" } =>
    search.plan === "change" ? { plan: "change" } : {},
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
  const { plan } = Route.useSearch();
  const navigate = Route.useNavigate();

  // The URL owns whether the plan dialog is open, so the blocked-invite CTA can
  // open it from the team page by navigating here. `replace` keeps opening and
  // closing it out of the back-button history.
  function setChangePlanOpen(open: boolean) {
    void navigate({
      search: open ? { plan: "change" } : {},
      replace: true,
    });
  }

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
              <WorkspaceDetailsCard
                workspace={currentWorkspace}
                changePlanOpen={plan === "change"}
                onChangePlanOpenChange={setChangePlanOpen}
              />
              <DeleteWorkspaceCard workspace={currentWorkspace} />
            </>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

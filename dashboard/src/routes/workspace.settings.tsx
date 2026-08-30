import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import { DeleteWorkspaceCard } from "@/features/workspaces/components/delete-workspace-card";
import { WorkspaceSettingsNavigation } from "@/features/workspaces/components/workspace-settings-navigation";
import { TeamPanel } from "@/features/team/components/team-panel";
import { WorkspaceSettingsPageSkeleton } from "@/common/components/route-skeletons";
import { SECTION_NAVIGATION_STICKY_CLASS } from "@/common/components/section-navigation";

export const Route = createFileRoute("/workspace/settings")({
  staticData: { chrome: true },
  component: WorkspaceSettingsPage,
  pendingComponent: WorkspaceSettingsPageSkeleton,
  // No-arg requireAuth (w1/m45): `next` keeps the full href so the
  // `?plan=change` deep link (blocked-invite CTA, /billing/update-plan alias)
  // survives the SSR login bounce.
  beforeLoad: requireAuth(),
  // `?plan=change` arrives from the blocked-invite CTA (w6/m15/t001) and opens
  // the change-plan dialog straight away, so an invite the plan refused is one
  // click from the upgrade that would allow it. Advisory: anything else just
  // renders the settings page.
  // Optional, not `plan: undefined` — an always-present key would make `search`
  // mandatory on every navigation to this route (the switcher's included).
  validateSearch: (search: Record<string, unknown>): { plan?: "change" } =>
    search.plan === "change" ? { plan: "change" } : {},
  head: ({ match }) =>
    translatedTitleHead("workspaces.switcherSettings", match),
});

/**
 * Workspace settings (w6/m3/t003-t004): the currently selected workspace's
 * rename/plan/metadata card plus its delete danger zone — distinct from
 * `/settings` (account settings, Kratos-backed) since a workspace and a user
 * account are different objects with different owners.
 */
function WorkspaceSettingsPage() {
  const { t } = useTranslations();
  const { currentWorkspace, loading, workspaces } = useWorkspace();
  const { plan } = Route.useSearch();
  const navigate = Route.useNavigate();
  const hasMultipleWorkspaces = workspaces.length > 1;

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
        <div className="mx-auto grid w-full max-w-4xl items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
          {/* Same right-rail quick nav as the service settings page. */}
          <WorkspaceSettingsNavigation
            className={SECTION_NAVIGATION_STICKY_CLASS}
            showDangerZone={hasMultipleWorkspaces}
          />

          <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
            <div>
              <h1 className="text-xl font-semibold">
                {t("workspaces.settingsTitle")}
              </h1>
              <p className="text-sm text-muted-foreground">
                {t("workspaces.settingsDescription")}
              </p>
            </div>
            {!currentWorkspace && loading ? (
              <>
                <section data-skeleton-region="general">
                  <CardSkeleton rows={5} />
                </section>
                <section data-skeleton-region="team">
                  <CardSkeleton rows={5} />
                </section>
                {hasMultipleWorkspaces ? (
                  <section data-skeleton-region="danger-zone">
                    <CardSkeleton rows={2} />
                  </section>
                ) : null}
              </>
            ) : !currentWorkspace ? (
              <p className="text-muted-foreground text-sm">
                {t("workspaces.settingsEmpty")}
              </p>
            ) : (
              <>
                <section id="general" className="scroll-mt-6">
                  <WorkspaceDetailsCard
                    workspace={currentWorkspace}
                    changePlanOpen={plan === "change"}
                    onChangePlanOpenChange={setChangePlanOpen}
                  />
                </section>
                {/* Team management lives here (w1/m33/t006) — members are
                    workspace-scoped objects, and this is where Render puts them;
                    it lived on account /settings from w4/m12 until now. */}
                <section id="team" className="scroll-mt-6">
                  <TeamPanel />
                </section>
                {hasMultipleWorkspaces ? (
                  <section id="danger-zone" className="scroll-mt-6">
                    <DeleteWorkspaceCard workspace={currentWorkspace} />
                  </section>
                ) : null}
              </>
            )}
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}

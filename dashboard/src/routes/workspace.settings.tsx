import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import { WorkspaceIPRulesCard } from "@/features/workspaces/components/workspace-ip-rules-card";
import { WorkspaceLoginRequirementCard } from "@/features/workspaces/components/workspace-login-requirement-card";
import { WorkspacePrivacyCard } from "@/features/workspaces/components/workspace-privacy-card";
import { BuildPipelineCard } from "@/features/usage/components/build-pipeline-card";
import { OverlappingDeployPolicyCard } from "@/features/workspaces/components/overlapping-deploy-policy-card";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { DeleteWorkspaceCard } from "@/features/workspaces/components/delete-workspace-card";
import { WorkspaceSettingsNavigation } from "@/features/workspaces/components/workspace-settings-navigation";
import { TeamPanel } from "@/features/team/components/team-panel";
import { LeaveWorkspaceCard } from "@/features/team/components/leave-workspace-card";
import { AgentEnvironmentCard } from "@/features/agent-environment/components/agent-environment-card";
import { AgentSecretsCard } from "@/features/agent-environment/components/agent-secrets-card";
import { AgentRepoEnvironmentsCard } from "@/features/agent-environment/components/agent-repo-environments-card";
import { AgentSkillsCard } from "@/features/agent-environment/components/agent-skills-card";
import { WorkspaceNotificationsCard } from "@/features/notifications/components/workspace-notifications-card";
import { NotificationOverridesCard } from "@/features/notifications/components/notification-overrides-card";
import {
  WorkspaceSettingsPageSkeleton,
  WorkspaceSettingsSkeletonSections,
} from "@/common/components/route-skeletons";
import { useWorkspaceSettingsSkeletonSections } from "@/features/workspaces/hooks/use-workspace-settings-skeleton-sections";
import { RegistryCredentialsPanel } from "@/features/registry-credentials/components/registry-credentials-panel";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import { useAuditLog } from "@/features/audit/hooks/use-audit-log";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { cn } from "@/common/lib/utils/utils";

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
  // Typed optional (`plan?:`) so a navigation to this route (the switcher's
  // included) needn't pass `search`; the value always carries the key.
  validateSearch: (search: Record<string, unknown>): { plan?: "change" } => ({
    plan: search.plan === "change" ? "change" : undefined,
  }),
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
  const audit = useAuditLog();
  const capabilities = useCapabilities();
  // Audit logs are admin-only (`can_manage`). The list's role is "admin" for
  // every member, so capabilities decide once they answer: before the query's
  // 403, a non-admin never sees the section, its link or its skeleton flash,
  // and the pending frame drops it too (w7/094).
  const showAuditLogs =
    currentWorkspace?.role === "admin" &&
    !audit.forbidden &&
    (capabilities.canManage || capabilities.loading);
  // Build Pipeline (w6/m309) holds billing settings: billing managers only.
  const showBuildPipeline = capabilities.canManageBilling;
  // Workspace privacy (w7/m464) is an org workspace's, and its admins'
  // (`can_manage`; the workspace list's role says "admin" for everyone).
  const showPrivacy =
    currentWorkspace?.organization != null && capabilities.canManage;
  // While the workspace loads, the rail lists the sections the skeleton draws.
  const pending = !currentWorkspace && loading;
  const skeleton = useWorkspaceSettingsSkeletonSections();

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
      <PageFrame
        header={
          <PageHeader
            title={t("workspaces.settingsTitle")}
            description={t("workspaces.settingsDescription")}
          />
        }
        navigation={
          <WorkspaceSettingsNavigation
            showDangerZone
            showAuditLogs={pending ? skeleton.auditLogs : showAuditLogs}
            showBuildPipeline={
              pending ? skeleton.buildPipeline : showBuildPipeline
            }
            showPrivacy={pending ? skeleton.privacy : showPrivacy}
          />
        }
      >
        {pending ? (
          <WorkspaceSettingsSkeletonSections sections={skeleton} />
        ) : !currentWorkspace ? (
          <p className="text-muted-foreground text-sm">
            {t("workspaces.settingsEmpty")}
          </p>
        ) : (
          <>
            <section id="general" className={SECTION_NAVIGATION_TARGET_CLASS}>
              <WorkspaceDetailsCard
                workspace={currentWorkspace}
                changePlanOpen={plan === "change"}
                onChangePlanOpenChange={setChangePlanOpen}
              />
            </section>
            {/* Workspace privacy (w7/m464): Render's Public or Invite-only
                    for a workspace in an organization, set by its admins. */}
            {showPrivacy ? (
              <section id="privacy" className={SECTION_NAVIGATION_TARGET_CLASS}>
                <WorkspacePrivacyCard workspace={currentWorkspace} />
              </section>
            ) : null}
            {/* Workspace Settings → Build Pipeline (w6/m309), where Render
                    sets the monthly pipeline spend limit. */}
            {showBuildPipeline ? (
              <section
                id="build-pipeline"
                className={SECTION_NAVIGATION_TARGET_CLASS}
              >
                <BuildPipelineCard workspaceId={currentWorkspace.id} />
              </section>
            ) : null}
            {/* Workspace Settings → Overlapping Deploy Policy (w4/653),
                    after Build Pipeline as on Render: every member reads it
                    and Admins change it. */}
            <section
              id="overlapping-deploy-policy"
              className={SECTION_NAVIGATION_TARGET_CLASS}
            >
              <OverlappingDeployPolicyCard workspaceId={currentWorkspace.id} />
            </section>
            {/* Workspace Settings → Inbound IP Rules (w9/m388): Render's
                    workspace-level rules (Scale and Enterprise), every member
                    reads them and Admins edit them. */}
            <section
              id="inbound-ip-rules"
              className={SECTION_NAVIGATION_TARGET_CLASS}
            >
              <WorkspaceIPRulesCard
                workspace={currentWorkspace}
                onChangePlan={() => setChangePlanOpen(true)}
              />
            </section>
            {/* Render's "Enforce 2FA & Google login" (w22/m402, w7/m465):
                    every member reads both settings and Admins change them. */}
            <section
              id="two-factor"
              className={SECTION_NAVIGATION_TARGET_CLASS}
            >
              <WorkspaceLoginRequirementCard
                requirement="twoFactor"
                workspace={currentWorkspace}
                onChangePlan={() => setChangePlanOpen(true)}
              />
            </section>
            <section
              id="google-login"
              className={SECTION_NAVIGATION_TARGET_CLASS}
            >
              <WorkspaceLoginRequirementCard
                requirement="googleLogin"
                workspace={currentWorkspace}
                onChangePlan={() => setChangePlanOpen(true)}
              />
            </section>
            {/* Team management lives here (w1/m33/t006) — members are
                    workspace-scoped objects, and this is where Render puts them;
                    it lived on account /settings from w4/m12 until now. */}
            <section id="team" className={SECTION_NAVIGATION_TARGET_CLASS}>
              <TeamPanel />
            </section>
            {/* Settings → Agents (w10/m17/t005): the workspace agent
                    environment — network tier and default model — configured
                    once here instead of per session. */}
            {/* Settings → Notifications (w3/m271): the workspace
                    notification policy every service on "Use workspace
                    default" follows — Render's owner notification settings. */}
            <section
              id="notifications"
              className={cn(SECTION_NAVIGATION_TARGET_CLASS, "space-y-6")}
            >
              <WorkspaceNotificationsCard ownerId={currentWorkspace.id} />
              <NotificationOverridesCard ownerId={currentWorkspace.id} />
            </section>
            <section
              id="agents"
              className={cn(SECTION_NAVIGATION_TARGET_CLASS, "space-y-6")}
            >
              <AgentEnvironmentCard ownerId={currentWorkspace.id} />
              {/* Workspace agent secrets (ADR098 D8): names only. */}
              <AgentSecretsCard ownerId={currentWorkspace.id} />
              {/* Repository environments (ADR109): per repository, the
                      setup steps run before the agent and the checks it runs
                      before delivering. */}
              <AgentRepoEnvironmentsCard ownerId={currentWorkspace.id} />
              {/* Workspace skills (ADR099 D6): SKILL.md documents every
                      agent session receives. */}
              <AgentSkillsCard ownerId={currentWorkspace.id} />
            </section>
            <RegistryCredentialsPanel />
            {showAuditLogs ? (
              <section
                id="audit-logs"
                className={SECTION_NAVIGATION_TARGET_CLASS}
              >
                <AuditLogPanel state={audit} ownerId={currentWorkspace.id} />
              </section>
            ) : null}
            {/* The danger zone is now always present: Leave workspace
                    (w5/m102) lives here for every member, while Delete stays
                    gated on having somewhere else to land. The Leave card
                    renders nothing when the caller has no membership row. */}
            <section
              id="danger-zone"
              className={cn(SECTION_NAVIGATION_TARGET_CLASS, "space-y-6")}
            >
              <LeaveWorkspaceCard workspace={currentWorkspace} />
              {hasMultipleWorkspaces ? (
                <DeleteWorkspaceCard workspace={currentWorkspace} />
              ) : null}
            </section>
          </>
        )}
      </PageFrame>
    </DashboardLayout>
  );
}

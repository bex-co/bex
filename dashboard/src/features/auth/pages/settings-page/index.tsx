import { getRouteApi } from "@tanstack/react-router";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { useScrollToHashedCard } from "@/common/hooks/use-scroll-to-hashed-card";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";
import { AccountDeletionCard } from "@/features/auth/pages/settings-page/account-deletion-card";
import { AccountSettingsFlow } from "@/features/auth/pages/settings-page/account-settings-flow";
import { AccountSettingsFlowSkeleton } from "@/features/auth/pages/settings-page/account-settings-flow-skeleton";
import { AppearanceCard } from "@/features/auth/pages/settings-page/appearance-card";
import { AccountAvatarCard } from "@/features/auth/pages/settings-page/account-avatar-card";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { SettingsNavigation } from "@/features/auth/pages/settings-page/settings-navigation";
import { SettingsSection } from "@/features/auth/pages/settings-page/settings-section";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";
import { SSHKeysPanel } from "@/features/ssh-keys/components/ssh-keys-panel";

const settingsRoute = getRouteApi("/settings");

/**
 * Render's account anchor for a section this page names otherwise (w3/m707):
 * its Account Security (password, sign-in methods, two-factor) is the Kratos
 * flow's cards under bex's Account section.
 */
const ACCOUNT_HASH_ALIASES: Readonly<Record<string, string>> = {
  "account-security": "account",
};

/**
 * Account settings — Kratos's settings flow (profile + password). This is
 * also where the recovery flow (forgot-password) lands once its code is
 * verified, since Kratos redirects a completed recovery straight to
 * `settings_ui_url` to let the user set a new password.
 */
export default function SettingsPage() {
  const search = settingsRoute.useSearch();
  const flow = useOryFlow("settings", search.flow);
  const { t } = useTranslations();
  useScrollToHashedCard("/settings", true, ACCOUNT_HASH_ALIASES);

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={t("auth.settingsTitle")}
            description={t("auth.settingsSubtitle")}
          />
        }
        navigation={<SettingsNavigation />}
      >
        <div className="space-y-10">
          <SettingsSection
            id="account"
            title={t("auth.accountSection")}
            description={t("auth.accountSectionSubtitle")}
          >
            {flow ? (
              <AccountSettingsFlow flow={flow} />
            ) : (
              <AccountSettingsFlowSkeleton />
            )}
            {/* Render's Avatar row follows the profile (w1/m648). */}
            <AccountAvatarCard />
          </SettingsSection>

          {/* Render's Appearance, after the profile as Render orders it
              (w3/m528); a log explorer's Options ▸ Theme Settings opens it. */}
          <SettingsSection
            id="appearance"
            title={t("auth.appearanceSection")}
            description={t("auth.appearanceSectionSubtitle")}
          >
            <AppearanceCard />
          </SettingsSection>

          <SettingsSection
            id="integrations"
            title={t("auth.integrationsSection")}
            description={t("auth.integrationsSectionSubtitle")}
          >
            <ConnectGithubCard
              callbackError={search.git_error}
              claimSelectionId={search.git_claim_selection}
            />
          </SettingsSection>

          <SettingsSection
            id="access"
            title={t("auth.accessSection")}
            description={t("auth.accessSectionSubtitle")}
          >
            <ApiKeysPanel />
            <SSHKeysPanel />
          </SettingsSection>

          <SecurityComplianceSection />

          <SettingsSection
            id="danger-zone"
            title={t("auth.dangerZoneSection")}
            description={t("auth.dangerZoneSectionSubtitle")}
          >
            <AccountDeletionCard />
          </SettingsSection>
        </div>
      </PageFrame>
    </DashboardLayout>
  );
}

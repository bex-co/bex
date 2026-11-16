import { getRouteApi } from "@tanstack/react-router";
import { Settings } from "@ory/elements-react/theme";
import { SessionProvider } from "@ory/elements-react/client";
import type { OryFlowComponentOverrides } from "@ory/elements-react";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import {
  KRATOS_PUBLIC_URL,
  useOryConfig,
  oryHideSettingsPageHeader,
} from "@/common/lib/ory/config";
import { OryToast } from "@/common/lib/ory/ory-toast";
import { orySettingsFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";
import { AccountDeletionCard } from "@/features/auth/pages/settings-page/account-deletion-card";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { SettingsNavigation } from "@/features/auth/pages/settings-page/settings-navigation";
import { SettingsSection } from "@/features/auth/pages/settings-page/settings-section";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";
import { SSHKeysPanel } from "@/features/ssh-keys/components/ssh-keys-panel";

const settingsRoute = getRouteApi("/settings");

const settingsComponents: OryFlowComponentOverrides = {
  ...oryHideSettingsPageHeader,
  ...orySettingsFormOverrides,
  Message: { Toast: OryToast },
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
  const oryConfig = useOryConfig();

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
              // Without baseUrl, SessionProvider guesses the Ory SDK URL as this
              // app's own origin — but Kratos lives on its own host, so its
              // whoami would 404/500 against the dashboard itself.
              <SessionProvider baseUrl={KRATOS_PUBLIC_URL}>
                <Settings
                  className="account-settings-flow"
                  flow={flow}
                  config={oryConfig}
                  components={settingsComponents}
                />
              </SessionProvider>
            ) : (
              <AccountSettingsSkeleton />
            )}
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

function AccountSettingsSkeleton() {
  return (
    <div aria-hidden="true" className="space-y-4">
      {["profile", "password"].map((section) => (
        <Card key={section} className="gap-4">
          <CardHeader className="gap-2">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-4 w-56 max-w-full" />
          </CardHeader>
          <CardContent className="space-y-3">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-28" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

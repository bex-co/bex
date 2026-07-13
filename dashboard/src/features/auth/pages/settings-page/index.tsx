import { useSearch } from "@tanstack/react-router";
import { Settings } from "@ory/elements-react/theme";
import { SessionProvider } from "@ory/elements-react/client";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import {
  useOryConfig,
  oryHideSettingsPageHeader,
} from "@/common/lib/ory/config";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { ApiKeysPanel } from "@/features/api-keys/components/api-keys-panel";
import { NotificationSettingsPanel } from "@/features/notifications/components/notification-settings-panel";
import { TeamPanel } from "@/features/team/components/team-panel";
import { SecurityComplianceSection } from "@/features/auth/pages/settings-page/security-compliance-section";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";

/**
 * Account settings — Kratos's settings flow (profile + password). This is
 * also where the recovery flow (forgot-password) lands once its code is
 * verified, since Kratos redirects a completed recovery straight to
 * `settings_ui_url` to let the user set a new password.
 */
export default function SettingsPage() {
  const search = useSearch({ strict: false }) as { flow?: string };
  const flow = useOryFlow("settings", search.flow);
  const { t } = useTranslations();
  const oryConfig = useOryConfig();

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <div className="space-y-1">
            <h1 className="text-2xl font-bold text-foreground">
              {t("auth.settingsTitle")}
            </h1>
            <p className="text-muted-foreground">
              {t("auth.settingsSubtitle")}
            </p>
          </div>
          {flow ? (
            <SessionProvider>
              <Settings
                flow={flow}
                config={oryConfig}
                components={oryHideSettingsPageHeader}
              />
            </SessionProvider>
          ) : (
            <div className="space-y-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          )}
          <TeamPanel />
          <ConnectGithubCard />
          <ApiKeysPanel />
          <NotificationSettingsPanel />
          <SecurityComplianceSection />
        </div>
      </div>
    </DashboardLayout>
  );
}

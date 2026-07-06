import { useSearch } from "@tanstack/react-router";
import { Settings } from "@ory/elements-react/theme";
import { SessionProvider } from "@ory/elements-react/client";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig, oryHideSettingsPageHeader } from "@/common/lib/ory/config";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Skeleton } from "@/common/components/ui/skeleton";

/**
 * Account settings — Kratos's settings flow (profile + password). This is
 * also where the recovery flow (forgot-password) lands once its code is
 * verified, since Kratos redirects a completed recovery straight to
 * `settings_ui_url` to let the user set a new password.
 */
export default function SettingsPage() {
  const search = useSearch({ strict: false }) as { flow?: string };
  const flow = useOryFlow("settings", search.flow);

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <div className="space-y-1">
            <h1 className="text-2xl font-bold text-foreground">Settings</h1>
            <p className="text-muted-foreground">
              Manage your account profile and password.
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
        </div>
      </div>
    </DashboardLayout>
  );
}

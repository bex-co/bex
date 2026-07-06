import { useSearch } from "@tanstack/react-router";
import { Settings } from "@ory/elements-react/theme";
import { SessionProvider } from "@ory/elements-react/client";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig } from "@/common/lib/ory/config";
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
      <div className="flex flex-col items-center gap-8 px-6 py-10 sm:px-10">
        <div className="w-full max-w-2xl">
          {flow ? (
            <SessionProvider>
              <Settings flow={flow} config={oryConfig} />
            </SessionProvider>
          ) : (
            <div className="space-y-4">
              <Skeleton className="h-8 w-64" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

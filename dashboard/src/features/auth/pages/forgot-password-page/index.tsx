import { useSearch } from "@tanstack/react-router";
import { Recovery } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

/**
 * Forgot-password page — Kratos's recovery flow. Submitting an email sends a
 * recovery code; entering the code establishes a privileged session and
 * redirects to the settings flow (see reset-password-page) to set a new
 * password.
 */
export default function ForgotPasswordPage() {
  const search = useSearch({ from: "/auth/forgot-password" });
  const flow = useOryFlow("recovery", search.flow);
  const { t } = useTranslations();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.forgotPasswordTitle")}
      subtitle={t("auth.forgotPasswordSubtitle")}
    >
      {flow ? (
        <Recovery flow={flow} config={oryConfig} components={oryHideCardLogo} />
      ) : (
        <div className="space-y-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
    </AuthPageShell>
  );
}

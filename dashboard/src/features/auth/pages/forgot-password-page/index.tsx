import { useSearch } from "@tanstack/react-router";
import { Recovery } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
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

  return (
    <AuthPageShell
      title="Reset your password"
      subtitle="Enter your email to receive a recovery code"
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

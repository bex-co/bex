import { useSearch } from "@tanstack/react-router";
import { Recovery } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { oryConfig } from "@/common/lib/ory/config";
import { Card, CardContent } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";

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
    <div className="min-h-screen flex items-center justify-center bg-background py-12 px-4 sm:px-6 lg:px-8">
      <div className="w-full max-w-md">
        <Card>
          <CardContent className="pt-6">
            {flow ? (
              <Recovery flow={flow} config={oryConfig} />
            ) : (
              <div className="space-y-4">
                <Skeleton className="h-8 w-64 mx-auto" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

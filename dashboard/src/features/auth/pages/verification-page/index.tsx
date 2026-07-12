import { useSearch } from "@tanstack/react-router";
import { Verification } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

/**
 * Verification page — Kratos's verification flow (docs/ADR012-auth.md §11). Registration
 * sends a one-time code to the new address; entering it here — or following the
 * emailed link, which arrives with `?flow=` (adopted + scrubbed by useOryFlow) —
 * marks the address verified. No auth guard: a just-registered user may not hold
 * a session yet, and the email link can be opened from anywhere.
 */
export default function VerificationPage() {
  const search = useSearch({ from: "/auth/verification" });
  const flow = useOryFlow("verification", search.flow);
  const { t } = useTranslations();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.verificationTitle")}
      subtitle={t("auth.verificationSubtitle")}
    >
      {flow ? (
        <Verification
          flow={flow}
          config={oryConfig}
          components={oryHideCardLogo}
        />
      ) : (
        <div className="space-y-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
    </AuthPageShell>
  );
}

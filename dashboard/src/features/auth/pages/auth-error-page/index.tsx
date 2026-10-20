import { useEffect, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { AuthStatusCard } from "@/features/auth/components/auth-status-card";

// The Kratos error ids a sign-in commonly ends on, by their translated
// explanation; any other id shows Kratos's own reason.
const KNOWN_ERRORS: Record<string, string> = {
  self_service_flow_expired: "auth.errorFlowExpired",
  security_csrf_violation: "auth.errorCsrf",
  security_identity_mismatch: "auth.errorIdentityMismatch",
  self_service_flow_return_to_forbidden: "auth.errorReturnToForbidden",
};

type KratosError = { id?: string; reason?: string; message?: string };

/**
 * `/auth/error` — where Kratos sends a self-service error it cannot show in
 * its flow (`selfservice.flows.error.ui_url`): an expired login or sign-up, a
 * CSRF mismatch, a forbidden return address. It reads the error by its `id`
 * — client-side, as Kratos answers it only with the browser's cookies — and
 * says what went wrong, with the way back to sign in. An unknown or missing
 * id reads as the generic failure.
 */
export default function AuthErrorPage({ id }: { id?: string }) {
  const { t } = useTranslations();
  // undefined while Kratos is asked; null once it has no readable answer.
  const [error, setError] = useState<KratosError | null | undefined>(
    id ? undefined : null,
  );

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    void createFrontendApi()
      .getFlowError({ id })
      .then((flowError) => (flowError.error as KratosError | undefined) ?? null)
      .catch(() => null)
      .then((found) => {
        if (!cancelled) setError(found);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  const known = error?.id ? KNOWN_ERRORS[error.id] : undefined;
  const message =
    error === undefined ? (
      <Skeleton className="mt-0.5 h-4 w-64" />
    ) : known ? (
      t(known)
    ) : (
      error?.reason || error?.message || t("auth.errorGeneric")
    );

  return (
    <AuthPageShell
      title={t("auth.errorTitle")}
      subtitle={t("auth.errorSubtitle")}
    >
      <AuthStatusCard
        icon={
          <AlertTriangle className="mt-0.5 size-6 shrink-0 text-destructive" />
        }
        message={message}
        action={
          <Button asChild variant="outline">
            <Link to="/auth/login" search={EMPTY_LOGIN_SEARCH}>
              {t("auth.errorBackToSignIn")}
            </Link>
          </Button>
        }
      />
    </AuthPageShell>
  );
}

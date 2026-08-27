import { AlertTriangle, LogIn } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useSignInAgain } from "@/common/lib/auth/sign-in-again";

/**
 * Settled-error state for a detail page whose resource query failed (w9/m55).
 * Distinct from not-found — a dead id redirects home via `useNotFoundRedirect`;
 * a failed query stays put here so a backend outage never masquerades as a
 * deleted resource.
 *
 * Two variants (w3/m80 t002), so an expired session never wears the network
 * error's clothes:
 * - `"error"` (default): a genuine outage — "check the API", with a Retry.
 * - `"unauthenticated"`: a 401 — "your session has expired", with Sign in. The
 *   Apollo auth link is usually already redirecting; this is the honest inline
 *   state until it lands, and the manual recovery if it doesn't.
 */
export function ResourceLoadError({
  onRetry,
  variant = "error",
}: {
  onRetry?: () => void;
  variant?: "error" | "unauthenticated";
}) {
  const { t } = useTranslations();
  const signInAgain = useSignInAgain();

  if (variant === "unauthenticated") {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-center">
        <LogIn className="size-8 text-muted-foreground" />
        <p className="font-medium">{t("common.sessionExpiredTitle")}</p>
        <p className="text-sm text-muted-foreground">
          {t("common.sessionExpiredBody")}
        </p>
        <Button size="sm" onClick={signInAgain}>
          {t("common.signIn")}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-2 py-12 text-center">
      <AlertTriangle className="size-8 text-destructive" />
      <p className="font-medium">{t("common.errorTitle")}</p>
      <p className="text-sm text-muted-foreground">
        {t("common.resourceErrorBody")}
      </p>
      {onRetry ? (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("common.tryAgain")}
        </Button>
      ) : null}
    </div>
  );
}

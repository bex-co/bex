import { useCallback, useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import {
  LogOut,
  Loader2,
  CheckCircle,
  AlertTriangle,
  type LucideIcon,
} from "lucide-react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { cn } from "@/common/lib/utils/utils.ts";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

type LogoutStatus = "confirm" | "logging-out" | "success" | "error";

/** Title/subtitle/icon for each logout state, so the page reads the same
 * shell chrome (theme, language switcher, typography) as every other auth
 * page (w10/m8 t003) instead of its own hand-rolled full-screen layout. */
function logoutStatusCopy(
  status: LogoutStatus,
  t: ReturnType<typeof useTranslations>["t"],
): { title: string; subtitle: string; icon: LucideIcon; tone: "default" | "error" } {
  switch (status) {
    case "logging-out":
      return {
        title: t("auth.loggingOutTitle"),
        subtitle: t("auth.loggingOutSubtitle"),
        icon: Loader2,
        tone: "default",
      };
    case "success":
      return {
        title: t("auth.loggedOutTitle"),
        subtitle: t("auth.loggedOutSubtitle"),
        icon: CheckCircle,
        tone: "default",
      };
    case "error":
      return {
        title: t("auth.logoutFailedTitle"),
        subtitle: t("auth.logoutFailedSubtitle"),
        icon: AlertTriangle,
        tone: "error",
      };
    default:
      return {
        title: t("auth.logoutConfirmTitle"),
        subtitle: t("auth.logoutConfirmSubtitle"),
        icon: LogOut,
        tone: "default",
      };
  }
}

/**
 * Logout page — calls Kratos's browser logout flow (which clears the
 * `ory_kratos_session` cookie) then redirects to login.
 *
 * SECURITY (codex #12): logout requires an explicit click — the page renders a
 * confirmation first and only calls Kratos when the user presses the button. This
 * prevents a cross-site top-level navigation from silently ending the victim's
 * session (CSRF logout). A `useEffect`-driven auto-logout on GET would let any
 * malicious site link to /auth/logout and log the user out.
 *
 * SECURITY (codex #6): the provider-side session is the real boundary, so we
 * treat the flow as done when Kratos returns success OR reports there is
 * already no session (401/403 — unsigned visitor). A true provider failure
 * (5xx/network) keeps a blocking error with a retry — never a "signed out"
 * screen while the HttpOnly cookie might still be valid. Local cache clearing
 * and navigation happen only on real success / already-signed-out
 * (`endBrowserSession` in `@/common/lib/ory/logout`).
 */
export default function LogoutPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const { t } = useTranslations();
  const [status, setStatus] = useState<LogoutStatus>("confirm");

  const performLogout = useCallback(async () => {
    try {
      setStatus("logging-out");
      await endBrowserSession();
      await router.invalidate();

      setStatus("success");
      void navigate({ to: "/auth/login", search: EMPTY_LOGIN_SEARCH });
    } catch (error) {
      console.error("Logout failed:", error);
      setStatus("error");
    }
  }, [navigate, router]);

  const { title, subtitle, icon: Icon, tone } = logoutStatusCopy(status, t);

  return (
    <AuthPageShell title={title} subtitle={subtitle}>
      <Card>
        <CardContent className="flex flex-col items-center gap-6 py-8">
          <div className="relative">
            <div
              className={cn(
                "absolute inset-0 rounded-full",
                tone === "error" ? "bg-destructive/20" : "bg-primary/20",
              )}
            />
            <div
              className={cn(
                "relative rounded-full p-6",
                tone === "error" ? "bg-destructive" : "bg-primary",
              )}
            >
              <Icon
                className={cn(
                  "h-10 w-10 text-primary-foreground",
                  status === "logging-out" && "animate-spin",
                )}
              />
            </div>
          </div>
          {status === "confirm" && (
            <div className="flex items-center justify-center gap-3">
              <Button
                variant="outline"
                onClick={() => void navigate({ to: "/" })}
              >
                {t("auth.logoutCancel")}
              </Button>
              <Button onClick={() => void performLogout()}>
                <LogOut className="mr-2 h-4 w-4" />
                {t("auth.logoutConfirm")}
              </Button>
            </div>
          )}
          {status === "error" && (
            <Button onClick={() => void performLogout()}>
              {t("auth.logoutRetry")}
            </Button>
          )}
        </CardContent>
      </Card>
    </AuthPageShell>
  );
}

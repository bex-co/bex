import { useCallback, useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { LogOut, Loader2, CheckCircle, AlertTriangle } from "lucide-react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";

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
 * treat the flow as done ONLY when Kratos returns a successful response. A failed
 * or errored logout keeps a blocking error with a retry — never a "signed out"
 * screen or a redirect to login — because presenting success while the HttpOnly
 * Kratos cookie is still valid would let the next user of this browser inherit
 * the session. Local cache clearing and navigation happen only on real success.
 */
export default function LogoutPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const { t } = useTranslations();
  const [status, setStatus] = useState<
    "confirm" | "logging-out" | "success" | "error"
  >("confirm");

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

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-background to-muted/20">
      <div className="space-y-8">
        <div className="flex justify-center">
          <div className="relative">
            <div
              className={`absolute inset-0 rounded-full ${status === "error" ? "bg-destructive/20" : "bg-primary/20"}`}
            />
            <div
              className={`relative rounded-full p-6 ${status === "error" ? "bg-destructive" : "bg-primary"}`}
            >
              {status === "error" ? (
                <AlertTriangle className="h-12 w-12 text-primary-foreground" />
              ) : (
                <LogOut className="h-12 w-12 text-primary-foreground" />
              )}
            </div>
          </div>
        </div>

        <div className="text-center space-y-3">
          {status === "confirm" && (
            <>
              <h1 className="text-2xl font-semibold">
                {t("auth.logoutConfirmTitle")}
              </h1>
              <p className="text-muted-foreground">
                {t("auth.logoutConfirmSubtitle")}
              </p>
              <div className="flex items-center justify-center gap-3 pt-2">
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
            </>
          )}
          {status === "logging-out" && (
            <>
              <div className="flex items-center justify-center gap-3">
                <Loader2 className="h-6 w-6 animate-spin text-primary" />
                <h1 className="text-2xl font-semibold">
                  {t("auth.loggingOutTitle")}
                </h1>
              </div>
              <p className="text-muted-foreground">
                {t("auth.loggingOutSubtitle")}
              </p>
            </>
          )}
          {status === "success" && (
            <>
              <div className="flex items-center justify-center gap-3">
                <CheckCircle className="h-6 w-6 text-green-500 animate-in zoom-in duration-200" />
                <h1 className="text-2xl font-semibold text-green-500">
                  {t("auth.loggedOutTitle")}
                </h1>
              </div>
              <p className="text-muted-foreground">
                {t("auth.loggedOutSubtitle")}
              </p>
            </>
          )}
          {status === "error" && (
            <>
              <h1 className="text-2xl font-semibold text-destructive">
                {t("auth.logoutFailedTitle")}
              </h1>
              <p className="text-muted-foreground">
                {t("auth.logoutFailedSubtitle")}
              </p>
              <Button onClick={() => void performLogout()} className="mt-2">
                {t("auth.logoutRetry")}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

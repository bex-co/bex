import { useCallback, useEffect, useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { LogOut, Loader2, CheckCircle, AlertTriangle } from "lucide-react";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { getClient } from "@/common/apollo/client";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";

/**
 * Logout page — calls Kratos's browser logout flow (which clears the
 * `ory_kratos_session` cookie) then redirects to login.
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
  const [status, setStatus] = useState<"logging-out" | "success" | "error">(
    "logging-out",
  );

  const performLogout = useCallback(async () => {
    try {
      setStatus("logging-out");

      const api = createFrontendApi();
      const { logout_url } = await api.createBrowserLogoutFlow();
      const response = await fetch(logout_url, { credentials: "include" });
      // fetch resolves on 4xx/5xx, so an unchecked response would hide a failed
      // logout. Require a successful provider response before treating the
      // session as ended.
      if (!response.ok) {
        throw new Error(`logout request failed: ${response.status}`);
      }

      // Provider session is cleared — now it is safe to drop cached
      // account-scoped data (the CSR Apollo client is a module singleton that
      // survives logout, so without this the next account could read the
      // previous one's cached workspaces/resources, codex-security #24) and
      // leave the page.
      invalidateSessionCache();
      void getClient().clearStore();
      await router.invalidate();

      setStatus("success");
      void navigate({ to: "/auth/login", search: EMPTY_LOGIN_SEARCH });
    } catch (error) {
      console.error("Logout failed:", error);
      setStatus("error");
    }
  }, [navigate, router]);

  useEffect(() => {
    void performLogout();
  }, [performLogout]);

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

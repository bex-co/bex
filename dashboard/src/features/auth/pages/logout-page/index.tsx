import { useEffect, useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { LogOut, Loader2, CheckCircle } from "lucide-react";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Logout page — calls Kratos's browser logout flow (which clears the
 * `ory_kratos_session` cookie) then redirects to login.
 */
export default function LogoutPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const { t } = useTranslations();
  const [status, setStatus] = useState<"logging-out" | "success">(
    "logging-out",
  );

  useEffect(() => {
    const performLogout = async () => {
      try {
        setStatus("logging-out");
        await new Promise((resolve) => setTimeout(resolve, 800));

        const api = createFrontendApi();
        const { logout_url } = await api.createBrowserLogoutFlow();
        await fetch(logout_url, { credentials: "include" });

        setStatus("success");
        await new Promise((resolve) => setTimeout(resolve, 600));
      } catch (error) {
        console.error("Logout failed:", error);
      } finally {
        invalidateSessionCache();
        await router.invalidate();
        void navigate({
          to: "/auth/login",
          search: EMPTY_LOGIN_SEARCH,
        });
      }
    };

    void performLogout();
  }, [navigate, router]);

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-background to-muted/20">
      <div className="space-y-8">
        <div className="flex justify-center">
          <div className="relative">
            <div className="absolute inset-0 bg-primary/20 rounded-full" />
            <div className="relative bg-primary rounded-full p-6">
              <LogOut className="h-12 w-12 text-primary-foreground" />
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
        </div>
      </div>
    </div>
  );
}

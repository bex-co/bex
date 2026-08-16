import { useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Registration } from "@ory/elements-react/theme";
import { useOryFlow, clearStoredOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { getClient } from "@/common/apollo/client";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { useAuthFeatures } from "@/features/auth/components/auth-page-shell/auth-features";

export default function RegisterPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const search = useSearch({ from: "/auth/sign-up" });
  const flow = useOryFlow("registration", search.flow, {
    loginChallenge: search.login_challenge,
  });
  const { t } = useTranslations();
  const authFeatures = useAuthFeatures();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.registerTitle")}
      subtitle={t("auth.registerSubtitle")}
      features={authFeatures}
    >
      {flow ? (
        <Registration
          flow={flow}
          config={oryConfig}
          components={oryHideCardLogo}
          onSuccess={async () => {
            clearStoredOryFlow("registration");
            // Registration is an authenticated-subject transition just like
            // login. The CSR Apollo client is a tab-lifetime singleton, so clear
            // any prior account's normalized data before the new session can
            // render authenticated routes.
            await getClient().clearStore();
            // The root route's beforeLoad cached the (unauthenticated)
            // session on first load — force it to refetch before navigating
            // to an authenticated route, or requireAuth bounces us right
            // back to /auth/login on stale context.
            invalidateSessionCache();
            await router.invalidate();
            void navigate({ to: "/" });
          }}
        />
      ) : (
        <div className="space-y-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
    </AuthPageShell>
  );
}

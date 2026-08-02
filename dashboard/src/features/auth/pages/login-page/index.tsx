import { useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Login } from "@ory/elements-react/theme";
import { useOryFlow, clearStoredOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig, oryHideCardLogo } from "@/common/lib/ory/config";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { getClient } from "@/common/apollo/client";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { useAuthFeatures } from "@/features/auth/components/auth-page-shell/auth-features";

export default function LoginPage() {
  const navigate = useNavigate();
  const router = useRouter();
  const search = useSearch({ from: "/auth/login" });
  const flow = useOryFlow("login", search.flow, {
    returnTo: search.next || "/",
    loginChallenge: search.login_challenge,
    aal: search.aal,
  });
  const { t } = useTranslations();
  const authFeatures = useAuthFeatures();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.loginTitle")}
      subtitle={t("auth.loginSubtitle")}
      features={authFeatures}
    >
      {flow ? (
        <Login
          flow={flow}
          config={oryConfig}
          components={oryHideCardLogo}
          onSuccess={async () => {
            clearStoredOryFlow("login");
            // Drop any data cached for a prior account before the new session
            // begins — the CSR Apollo client is a module singleton that survives
            // logout/login, so without this the next account could read the
            // previous one's cached workspaces/resources (codex-security #24).
            await getClient().clearStore();
            // See register-page: root's beforeLoad cached the (unauthenticated)
            // session on first load — refetch it before navigating.
            invalidateSessionCache();
            await router.invalidate();
            // `next` goes in `href`, not `to`: it is an arbitrary href and may
            // carry a query string (the OAuth consent bounce sends
            // `/auth/consent?consent_challenge=…`), which `to` would swallow
            // into the pathname. `href` wins over `to` when set, so `to` is
            // just the no-`next` fallback.
            void navigate({ to: "/", href: search.next });
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

import { useEffect } from "react";
import { useNavigate, useRouter, useSearch } from "@tanstack/react-router";
import { Login } from "@ory/elements-react/theme";
import { useOryFlow, clearStoredOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig } from "@/common/lib/ory/config";
import { oryAuthFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { safeNext } from "@/common/lib/safe-next";
import { stashAuthNext, clearAuthNext } from "@/features/auth/lib/auth-next";
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
  // `next` is attacker-controllable (it's in the URL). Normalize it to a
  // same-origin relative path before it becomes the Ory return_to OR a
  // navigate href — otherwise it's an open redirect (safe-next.ts).
  const nextTarget = safeNext(search.next);
  const flow = useOryFlow("login", search.flow, {
    returnTo: nextTarget,
    loginChallenge: search.login_challenge,
    aal: search.aal,
  });
  const { t } = useTranslations();
  const authFeatures = useAuthFeatures();
  const oryConfig = useOryConfig();

  // ADR075 D8 (w6/m42): an unverified identity's login attempt is refused by
  // Kratos's require_verified_address hook, and Ory Elements full-page-redirects
  // to /auth/verification?flow=… — a URL Kratos builds, which drops `?next=`.
  // Stash the guarded target so the verification page can carry it back to
  // login; a successful login clears the relay below (any stash is stale then).
  useEffect(() => {
    stashAuthNext(search.next);
  }, [search.next]);

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
          components={oryAuthFormOverrides}
          onSuccess={async () => {
            clearStoredOryFlow("login");
            clearAuthNext();
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
            // just the fallback. `nextTarget` is already same-origin-normalized.
            void navigate({ to: "/", href: nextTarget });
          }}
        />
      ) : (
        <LoginWidgetSkeleton />
      )}
    </AuthPageShell>
  );
}

/**
 * Placeholder shown while the Kratos login flow is being fetched. It mirrors the
 * structure the Ory <Login> card renders once the flow arrives — bordered card,
 * header (title + subtitle), a social-provider button, a divider, labelled
 * e-mail + password fields, the submit button, and the sign-up footer — using
 * the same card chrome (rounded-xl border bg-card shadow-sm, p-6 sm:p-8) and the
 * same vertical rhythm (card gap-6, inner field gap-8, label gap-1). Matching
 * the real widget's box keeps the layout stable, so the form doesn't jump when
 * it swaps in. (Kept in sync by eye with the Ory card measured in the browser;
 * the card's own content-driven ~480px overflow width is left to the real card.)
 */
function LoginWidgetSkeleton() {
  return (
    <div
      aria-hidden
      className="grid gap-6 rounded-xl border bg-card p-6 shadow-sm sm:p-8"
    >
      {/* header: title + subtitle */}
      <div className="space-y-2">
        <Skeleton className="h-7 w-28" />
        <Skeleton className="h-6 w-4/5" />
      </div>
      {/* social provider button */}
      <Skeleton className="h-9 w-full" />
      {/* divider + e-mail/password fields + submit */}
      <div className="grid gap-8">
        <div className="h-px w-full bg-border" />
        <div className="flex flex-col gap-1">
          <div className="flex items-center justify-between">
            <Skeleton className="h-6 w-16" />
            <Skeleton className="h-6 w-32" />
          </div>
          <Skeleton className="h-9 w-full" />
        </div>
        <div className="flex flex-col gap-1">
          <Skeleton className="h-6 w-20" />
          <Skeleton className="h-9 w-full" />
        </div>
        <Skeleton className="h-9 w-full" />
      </div>
      {/* sign-up footer */}
      <Skeleton className="h-6 w-3/5" />
    </div>
  );
}

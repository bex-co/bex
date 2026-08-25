import { useSearch } from "@tanstack/react-router";
import { Registration } from "@ory/elements-react/theme";
import { useOryFlow, clearStoredOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig } from "@/common/lib/ory/config";
import { oryAuthFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { stashAuthNext } from "@/features/auth/lib/auth-next";
import { AuthWidgetSkeleton } from "@/common/components/route-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";
import { useAuthFeatures } from "@/features/auth/components/auth-page-shell/auth-features";

export default function RegisterPage() {
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
          components={oryAuthFormOverrides}
          onSuccess={() => {
            clearStoredOryFlow("registration");
            // ADR075 D3/D8 (w6/m42, revised 2026-08-20): registration mints a
            // session AND Kratos's continue_with routes to /auth/verification
            // (Ory Elements full-page-redirects there — show_verification_ui
            // outranks the plain redirect). NEVER navigate here: racing that
            // redirect would skip the verification step. The guarded `next`
            // rides the same-tab relay instead of the query string, because
            // the redirect URL is built by Kratos.
            stashAuthNext(search.next);
            // No cache invalidation here: Elements awaits onSuccess BEFORE its
            // continue_with redirect, and that redirect is a full page load
            // that rebuilds the Apollo/session state from scratch — awaited
            // cache work would only delay the signup hot path.
          }}
        />
      ) : (
        <AuthWidgetSkeleton fields={3} />
      )}
    </AuthPageShell>
  );
}

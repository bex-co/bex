import { useNavigate, useSearch } from "@tanstack/react-router";
import { FlowType, VerificationFlowState } from "@ory/client-fetch";
import { Verification } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { useOryConfig } from "@/common/lib/ory/config";
import { oryAuthFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { safeNext } from "@/common/lib/safe-next";
import { takeAuthNext } from "@/features/auth/lib/auth-next";
import { AuthWidgetSkeleton } from "@/common/components/route-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

/**
 * Verification page — Kratos's verification flow (docs/ADR012-auth.md §11). Registration
 * sends a one-time code to the new address; entering it here — or following the
 * emailed link, which arrives with `?flow=` (adopted + scrubbed by useOryFlow) —
 * marks the address verified. No auth guard: the email link can be opened from
 * anywhere, and the flow itself is unauthenticated by design. Under ADR075
 * D3/D8 (w6/m42, revised 2026-08-20) a just-registered user arrives HOLDING
 * the registration session, so success continues straight INTO the product —
 * the guarded `next` deep link (from `?next=` when linked directly, else the
 * same-tab relay the sign-up page stashed) or `/`. A session-less visitor (an
 * old email link in a fresh tab, or a stale unverified account bounced here by
 * the login backstop) takes the same navigation and requireAuth forwards them
 * to /auth/login with the same `next` preserved.
 */
export default function VerificationPage() {
  const navigate = useNavigate();
  const search = useSearch({ from: "/auth/verification" });
  const flow = useOryFlow("verification", search.flow);
  const { t } = useTranslations();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.verificationTitle")}
      subtitle={t("auth.verificationSubtitle")}
    >
      {flow ? (
        <Verification
          flow={flow}
          config={oryConfig}
          // oryAuthFormOverrides (not just the logo hide): brings the shared
          // input chrome AND the OTP code input that auto-submits on the
          // 6th digit (auth-form-overrides.tsx OtpCodeInput).
          components={oryAuthFormOverrides}
          onSuccess={(event) => {
            // onSuccess fires on every accepted submit — sending the address
            // (state "sent_email") as well as the final code. Only the code
            // acceptance completes verification.
            if (event.flowType !== FlowType.Verification) return;
            if (event.flow.state !== VerificationFlowState.PassedChallenge)
              return;
            // Continue INTO the product (the registration session is already
            // held); `next` goes in `href`, not `to` (see login-page): it may
            // carry a query string, and `href` wins over `to` when set. Both
            // sources are safeNext-normalized. A session-less visitor is
            // bounced by requireAuth to /auth/login with `next` preserved.
            const fromQuery = safeNext(search.next);
            // Consume the relay unconditionally so no stale stash outlives
            // this hop even when the query param wins.
            const stashed = takeAuthNext();
            const next = fromQuery !== "/" ? fromQuery : stashed;
            void navigate({ to: "/", href: next });
          }}
        />
      ) : (
        <AuthWidgetSkeleton fields={2} />
      )}
    </AuthPageShell>
  );
}

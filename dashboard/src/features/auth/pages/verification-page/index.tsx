import { useMemo } from "react";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { FlowType, VerificationFlowState } from "@ory/client-fetch";
import { Verification } from "@ory/elements-react/theme";
import { useOryFlow } from "@/common/hooks/use-ory-flow";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useOryConfig } from "@/common/lib/ory/config";
import { oryAuthFormOverrides } from "@/common/lib/ory/auth-form-overrides";
import { safeNext } from "@/common/lib/safe-next";
import { peekPendingInviteToken } from "@/common/lib/invite-token";
import { invalidateSessionCache } from "@/common/server-fn/session";
import { takeAuthNext } from "@/features/auth/lib/auth-next";
import { withPrefilledEmail } from "@/features/auth/lib/verification-prefill";
import { paymentSetupPath } from "@/features/onboarding/lib/payment-setup";
import {
  emailVerificationRequired,
  sessionTraitEmail,
} from "@/features/onboarding/lib/email-verification";
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
 * same-tab relay the sign-up page stashed) or `/` — via the sign-up payment
 * wall (`/setup/payment`, ADR075 D7 revised 2026-08-29), which forwards to
 * that target immediately when the workspace needs no card (gate off, exempt,
 * or already bound) and otherwise collects it as the last onboarding step. A
 * session-less visitor (an old email link in a fresh tab, or a stale
 * unverified account bounced here by the login backstop) takes the same
 * navigation and requireAuth forwards them to /auth/login with the wall (and
 * its `next`) preserved.
 *
 * It is also the verification WALL (ADR075 D8 revision 2026-10-06, w2/m168):
 * `EmailVerificationGate` and bex-api's `EMAIL_VERIFICATION_REQUIRED` backstop
 * send a signed-in unverified session here with `?next=` and no flow id. The
 * page then mints a fresh flow with the session's own email pre-filled (one
 * click sends the code; Kratos's code step offers resend), swaps the subtitle
 * to say why the user is here, and offers sign-out as the way out.
 */
export default function VerificationPage() {
  const navigate = useNavigate();
  const search = useSearch({ from: "/auth/verification" });
  const rawFlow = useOryFlow("verification", search.flow);
  const { session } = useRootContext();
  const walled = emailVerificationRequired(session);
  const email = sessionTraitEmail(session);
  const flow = useMemo(
    () => (rawFlow ? withPrefilledEmail(rawFlow, email) : null),
    [rawFlow, email],
  );
  const { t } = useTranslations();
  const oryConfig = useOryConfig();

  return (
    <AuthPageShell
      title={t("auth.verificationTitle")}
      subtitle={t(
        walled
          ? "auth.verificationRequiredSubtitle"
          : "auth.verificationSubtitle",
      )}
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
            // held) through the payment wall, which carries the deep link
            // onward; `href`, not `to` (see login-page): the wall URL carries
            // a query string, and `href` wins over `to` when set. Both `next`
            // sources are safeNext-normalized (again inside paymentSetupPath).
            // A session-less visitor is bounced by requireAuth to /auth/login
            // with the wall URL as `next`.
            const fromQuery = safeNext(search.next);
            // Consume the relay unconditionally so no stale stash outlives
            // this hop even when the query param wins.
            const stashed = takeAuthNext();
            const next = fromQuery !== "/" ? fromQuery : stashed;
            // The browser memoizes whoami for up to a minute. The address is
            // verified now, so drop the memo; otherwise the next root
            // beforeLoad would still see the unverified session, and
            // EmailVerificationGate would bounce the user back to this page.
            invalidateSessionCache();
            void navigate({
              to: "/",
              href: peekPendingInviteToken()
                ? "/invite"
                : paymentSetupPath(next),
            });
          }}
        />
      ) : (
        <AuthWidgetSkeleton fields={2} />
      )}
      {session ? (
        // Data-dependent (signed-in only), so the pending skeleton does not
        // reserve it. bex-api refuses every other exit for an unverified
        // human, so sign-out is the only one offered.
        <p className="text-sm text-muted-foreground">
          {t("auth.verificationWrongAccount")}{" "}
          <Link
            to="/auth/logout"
            className="font-medium text-foreground underline-offset-4 hover:underline"
          >
            {t("auth.verificationSignOut")}
          </Link>
        </p>
      ) : null}
    </AuthPageShell>
  );
}

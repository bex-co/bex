import { createFileRoute } from "@tanstack/react-router";
import { PaymentSetupRouteSkeleton } from "@/common/components/route-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import PaymentSetupPage from "@/features/onboarding/pages/payment-setup-page";

/**
 * The sign-up payment wall (ADR075 D7, revised 2026-08-29). Deliberately NOT
 * a chrome route: a workspace that cannot run anything yet gets the auth-page
 * shell (the same chrome as sign-up and verification — it is the last
 * onboarding step), not the sidebar, so there is nothing to wander off into.
 * `requireAuth`'s default `next` brings a session-less visitor straight back.
 */
export const Route = createFileRoute("/setup/payment")({
  component: PaymentSetupPage,
  pendingComponent: PaymentSetupRouteSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: (search: Record<string, unknown>) => ({
    // The guarded deep link to continue to once the gate opens (safeNext at
    // read; verification success and the root gate both set it).
    next: typeof search.next === "string" ? search.next : undefined,
    // Stripe Checkout's return state (useBillingOnboarding appends it).
    billing:
      search.billing === "success" || search.billing === "cancelled"
        ? (search.billing as "success" | "cancelled")
        : undefined,
  }),
  head: ({ match }) =>
    translatedTitleHead("onboarding.paymentSetupTitle", match),
});

import { BillingOnboardingView } from "@/features/usage/components/billing-onboarding";
import type { UseBillingOnboardingResult } from "@/features/usage/hooks/use-billing-onboarding";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

/**
 * Render-shaped Payment Method step on /new/workspace for paid plans. Reuses
 * the billing onboarding card against the *current* workspace (the new tea-*
 * does not exist yet). Does not attach a licensed workspace-fee Stripe Price.
 */
export function CreateWorkspacePaymentPanel({
  billing,
}: {
  billing: UseBillingOnboardingResult;
}) {
  const { t } = useTranslations();
  const { canManageBilling } = useCapabilities();

  return (
    <section className="space-y-3" aria-labelledby="workspace-payment-heading">
      <div className="space-y-1">
        <h2 id="workspace-payment-heading" className="text-lg font-semibold">
          {t("workspaces.paymentTitle")}
        </h2>
        <p className="text-muted-foreground text-sm">
          {t("workspaces.paymentDescription")}
        </p>
      </div>
      {billing.error && !billing.readiness ? null : (
        <BillingOnboardingView
          readiness={billing.readiness}
          loading={billing.loading}
          error={billing.error}
          checkoutBusy={billing.checkoutBusy}
          portalBusy={billing.portalBusy}
          onCheckout={() => void billing.openCheckout()}
          onPortal={() => void billing.openPortal()}
          canManageBilling={canManageBilling}
        />
      )}
    </section>
  );
}

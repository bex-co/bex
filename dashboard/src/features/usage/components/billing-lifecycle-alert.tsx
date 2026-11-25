import { CreditCard, ExternalLink, TriangleAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { safeHttpHref } from "@/common/lib/external-url";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import type { BillingReadiness } from "../hooks/use-billing-onboarding";
import { PaymentMethodRejectionAlert } from "./payment-method-rejection-alert";

/** The card's own Update payment method (hosted Checkout), offered again in
 *  the alert while an invoice is overdue. */
export interface BillingUpdatePaymentAction {
  onUpdatePayment: () => void;
  busy: boolean;
  /** Fail-closed; false disables the action. */
  canManageBilling: boolean;
  /** Disable-with-reason copy, as the card's own button shows it. */
  reason?: string;
}

/** The lifecycle states in which an invoice is overdue. */
const DUNNING_STATUSES = new Set(["grace", "enforcing", "enforced"]);

/**
 * What the member can do about an overdue invoice (w6/m626): pay it on
 * Stripe's hosted page, as Render's Pay Now does, or bind a card, which pays
 * it at once. Nothing when no invoice is overdue.
 */
function OverdueActions({
  readiness,
  update,
}: {
  readiness: BillingReadiness;
  update: BillingUpdatePaymentAction;
}) {
  const { t } = useTranslations();
  const { status, overdueInvoice } = readiness.lifecycle;
  if (!DUNNING_STATUSES.has(status) || !overdueInvoice) return null;
  const pay = safeHttpHref(overdueInvoice.hostedInvoiceUrl);
  return (
    <div className="mt-2 flex flex-wrap gap-2">
      {pay ? (
        <Button size="sm" asChild>
          <a href={pay} target="_blank" rel="noopener noreferrer">
            <CreditCard aria-hidden="true" />
            {t("usage.invoicePayNow")}
            <ExternalLink aria-hidden="true" />
          </a>
        </Button>
      ) : null}
      <PermissionTooltip reason={update.reason}>
        <Button
          size="sm"
          variant="outline"
          loading={update.busy}
          disabled={!update.canManageBilling}
          onClick={update.onUpdatePayment}
        >
          {t("usage.billingUpdatePayment")}
        </Button>
      </PermissionTooltip>
    </div>
  );
}

/** The workspace's billing lifecycle (grace, enforcement, recovery, exempt);
 *  nothing while it is healthy. */
function LifecycleAlert({
  readiness,
  update,
}: {
  readiness: BillingReadiness;
  update: BillingUpdatePaymentAction;
}) {
  const { t } = useTranslations();
  const state = readiness.lifecycle;
  if (state.status === "healthy") return null;
  let key = "usage.billingLifecycleUnknown";
  if (state.status === "grace") key = "usage.billingLifecycleGrace";
  if (state.status === "enforcing" || state.status === "enforced")
    key = "usage.billingLifecycleEnforced";
  if (state.status === "recovering") key = "usage.billingLifecycleRecovering";
  if (state.status === "excluded") key = "usage.billingLifecycleExcluded";
  if (state.status === "comped") key = "usage.billingLifecycleComped";
  return (
    <Alert
      variant={state.enforcementOwned ? "destructive" : "default"}
      data-testid="billing-lifecycle-alert"
    >
      <TriangleAlert aria-hidden="true" />
      <AlertDescription>
        <p>
          {t(key, {
            deadline: state.graceDeadline || t("usage.billingNoDeadline"),
            reason: state.reason || state.status,
          })}
        </p>
        <OverdueActions readiness={readiness} update={update} />
      </AlertDescription>
    </Alert>
  );
}

/**
 * The billing alerts both payment cards show: a card refused at binding
 * (ADR091 D10.5–D10.6), then the billing lifecycle, which carries Pay Now and
 * `update` while an invoice is overdue.
 */
export function BillingStatusAlerts({
  readiness,
  update,
}: {
  readiness: BillingReadiness;
  update: BillingUpdatePaymentAction;
}) {
  return (
    <>
      <PaymentMethodRejectionAlert reason={readiness.paymentMethodRejection} />
      <LifecycleAlert readiness={readiness} update={update} />
    </>
  );
}

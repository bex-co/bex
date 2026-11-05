import { TriangleAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import type { BillingReadiness } from "../hooks/use-billing-onboarding";
import { PaymentMethodRejectionAlert } from "./payment-method-rejection-alert";

/** The workspace's billing lifecycle (grace, enforcement, recovery, exempt);
 *  nothing while it is healthy. */
function LifecycleAlert({ readiness }: { readiness: BillingReadiness }) {
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
    <Alert variant={state.enforcementOwned ? "destructive" : "default"}>
      <TriangleAlert aria-hidden="true" />
      <AlertDescription>
        {t(key, {
          deadline: state.graceDeadline || t("usage.billingNoDeadline"),
          reason: state.reason || state.status,
        })}
      </AlertDescription>
    </Alert>
  );
}

/**
 * The billing alerts both payment cards show: a card refused at binding
 * (ADR091 D10.5–D10.6), then the billing lifecycle.
 */
export function BillingStatusAlerts({
  readiness,
}: {
  readiness: BillingReadiness;
}) {
  return (
    <>
      <PaymentMethodRejectionAlert reason={readiness.paymentMethodRejection} />
      <LifecycleAlert readiness={readiness} />
    </>
  );
}

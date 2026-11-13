import { Loader2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A deletion waiting for Stripe to finish aggregating the final usage
 * (BILLING_SETTLING, ADR091 D10.4); the flow retries on its own
 * (useSettlingRetry).
 */
export function BillingSettlingAlert() {
  const { t } = useTranslations();
  return (
    <Alert data-testid="billing-settling-alert" role="status">
      <Loader2 aria-hidden="true" className="animate-spin" />
      <AlertTitle>{t("common.billingSettlingTitle")}</AlertTitle>
      <AlertDescription>
        <p>{t("common.billingSettlingDescription")}</p>
      </AlertDescription>
    </Alert>
  );
}

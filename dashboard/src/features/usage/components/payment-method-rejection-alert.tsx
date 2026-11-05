import { TriangleAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { paymentMethodRejectionKey } from "../lib/payment-method-rejection";

/**
 * Why the last card offered for the workspace was refused at binding
 * (ADR091 D10.5–D10.6): a prepaid card, or one linked to an unpaid balance.
 * The refused card was never bound, so the workspace keeps its previous
 * method (or still has none). Renders nothing for no or an unknown reason.
 */
export function PaymentMethodRejectionAlert({
  reason,
}: {
  reason: string | null | undefined;
}) {
  const { t } = useTranslations();
  const key = paymentMethodRejectionKey(reason);
  if (!key) return null;
  return (
    <Alert variant="destructive" data-testid="payment-method-rejection">
      <TriangleAlert aria-hidden="true" />
      <AlertDescription>{t(key)}</AlertDescription>
    </Alert>
  );
}

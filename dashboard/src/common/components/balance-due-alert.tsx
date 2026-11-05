import { ExternalLink, TriangleAlert } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatMinorUnits } from "@/common/lib/format";
import type { BillingBalanceDue } from "@/common/lib/graphql-error";

export interface BalanceDueAlertProps {
  due: BillingBalanceDue;
  /** Names the workspace when the refusal is about one of several (account
   *  deletion); omitted on the workspace's own settings page. */
  workspaceName?: string;
}

/**
 * A deletion refused because settling the workspace's unbilled usage failed to
 * charge (BILLING_BALANCE_DUE, ADR091 D10.4): the balance and a link to
 * Stripe's hosted invoice page, in place of a generic error. Nothing was
 * deleted, so paying and retrying is the whole recovery.
 */
export function BalanceDueAlert({ due, workspaceName }: BalanceDueAlertProps) {
  const { t, i18n } = useTranslations();
  const amount =
    due.amountDueCents === null
      ? null
      : formatMinorUnits(due.amountDueCents, due.currency, i18n.language);
  let body: string;
  if (workspaceName) {
    body = amount
      ? t("common.balanceDueNamedAmount", { name: workspaceName, amount })
      : t("common.balanceDueNamed", { name: workspaceName });
  } else {
    body = amount
      ? t("common.balanceDueAmount", { amount })
      : t("common.balanceDue");
  }
  return (
    <Alert variant="destructive" data-testid="balance-due-alert">
      <TriangleAlert aria-hidden="true" />
      <AlertTitle>{t("common.balanceDueTitle")}</AlertTitle>
      <AlertDescription>
        <p>{body}</p>
        {due.hostedInvoiceUrl ? (
          <Button variant="outline" size="sm" className="mt-2" asChild>
            <a
              href={due.hostedInvoiceUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("common.balanceDuePay")}
              <ExternalLink aria-hidden="true" />
            </a>
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

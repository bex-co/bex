import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The "Open Billing" link a plan notice ends with (webhooks, request logs,
 * audit logs), so every Pro-feature notice sends the user to the same place
 * with the same copy (w8/m138).
 */
export function PlanBillingLink() {
  const { t } = useTranslations();
  return (
    <Link to="/billing" className="underline">
      {t("common.openBilling")}
    </Link>
  );
}

import { TableActionsHead } from "@/common/components/panel-states";
import { TableHead, TableHeader, TableRow } from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The webhook list's column labels, shared by the ready table and its loading
 * preview so the two cannot drift apart (w4/m220).
 */
export function WebhooksTableHeader() {
  const { t } = useTranslations();
  return (
    <TableHeader>
      <TableRow>
        <TableHead>{t("webhooks.colEndpoint")}</TableHead>
        <TableHead>{t("webhooks.colEvents")}</TableHead>
        <TableHead>{t("webhooks.colLatest")}</TableHead>
        <TableHead>{t("webhooks.colEnabled")}</TableHead>
        <TableActionsHead />
      </TableRow>
    </TableHeader>
  );
}

import {
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { MAX_EXPORT_DAYS } from "@/features/audit/lib/audit-csv";

/** The org name card's static header. */
export function OrganizationNameCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.nameTitle")}</CardTitle>
      <CardDescription>{t("organizations.nameDescription")}</CardDescription>
    </CardHeader>
  );
}

/** Org Settings → Audit Logs' static header (Render's audit-logs doc). */
export function OrganizationAuditLogsCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("organizations.auditTitle")}</CardTitle>
      <CardDescription>
        {t("organizations.auditDescription", { days: MAX_EXPORT_DAYS })}
      </CardDescription>
    </CardHeader>
  );
}

/** The name row: the field and, for an Owner, Save beside it. */
export const NAME_ROW_CLASS = "flex flex-col gap-3 sm:flex-row sm:items-end";

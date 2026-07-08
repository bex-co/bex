import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { deriveStatus } from "@/features/databases/lib/status";
import { STATUS_LABEL } from "@/features/databases/lib/labels";

/**
 * A database's status as a labeled badge: derive the status key from Render's
 * databaseStatus enum, look up its i18n label, render the matching variant. One
 * component for every place a status shows (list rows + detail header) so they
 * can't drift — mirrors services' ServiceStatusBadge.
 */
export function DatabaseStatusBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  const derived = deriveStatus({ status });
  return <Badge variant={derived.variant}>{t(STATUS_LABEL[derived.key])}</Badge>;
}

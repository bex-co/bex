import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { deriveStatus } from "@/features/databases/lib/status";
import { STATUS_LABEL } from "@/features/databases/lib/labels";

/**
 * A database's status as a labeled badge: derive the status key (suspension
 * wins over Render's databaseStatus enum — a suspended Postgres still reports
 * status "available"), look up its i18n label, render the matching variant.
 * Takes the whole view, not just `status`, so a call site can't drop the
 * suspended field and show a hibernated database as available. One component
 * for every place a status shows (list rows + detail header) so they can't
 * drift — mirrors services' ServiceStatusBadge.
 */
export function DatabaseStatusBadge({
  database,
}: {
  database: { status: string; suspended: string };
}) {
  const { t } = useTranslations();
  const derived = deriveStatus(database);
  return (
    <Badge variant={derived.variant}>{t(STATUS_LABEL[derived.key])}</Badge>
  );
}

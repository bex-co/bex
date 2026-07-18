import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { deriveStatus } from "@/features/keyvalue/lib/status";
import { STATUS_LABEL } from "@/features/keyvalue/lib/labels";

/**
 * A Key Value store's status as a labeled badge: derive the status key
 * (suspension wins over Render's keyValueStatus enum — a suspended store still
 * reports status "available"), look up its i18n label, render the matching
 * variant. Takes the whole view, not just `status`, so a call site can't drop
 * the suspended field and show a hibernated store as available. One component
 * for every place a status shows (list rows + detail header) so they can't
 * drift — mirrors databases' DatabaseStatusBadge.
 */
export function KeyValueStatusBadge({
  keyValue,
}: {
  keyValue: { status: string; suspended: boolean };
}) {
  const { t } = useTranslations();
  const derived = deriveStatus(keyValue);
  return (
    <Badge variant={derived.variant}>{t(STATUS_LABEL[derived.key])}</Badge>
  );
}

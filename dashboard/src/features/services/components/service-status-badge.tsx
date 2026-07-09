import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { deriveStatus } from "@/features/services/lib/status";
import { STATUS_LABEL } from "@/features/services/lib/labels";
import type { ServiceView } from "@/features/services/types";

/**
 * The service's status as a labeled badge: derive the status key (suspension
 * wins over phase), look up its i18n label, and render the matching badge
 * variant. One component for every place a status shows — the list rows, the
 * detail header, and the overview panel — so they can't drift. A sleeping
 * (auto-hibernated free-tier) App also carries the "wakes on the next request"
 * hint as a tooltip, so the explanation follows the badge everywhere.
 */
export function ServiceStatusBadge({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  const status = deriveStatus(service);
  const hint =
    status.key === "sleeping" ? t("services.statusSleepingHint") : undefined;
  return (
    <Badge variant={status.variant} title={hint}>
      {t(STATUS_LABEL[status.key])}
    </Badge>
  );
}

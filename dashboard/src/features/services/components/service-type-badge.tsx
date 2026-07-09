import { Badge } from "@/common/components/ui/badge.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  deriveServiceType,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";

/**
 * The service's kind (web / private / worker / cron) as a subtle outline badge —
 * one component for the list rows and the detail header so they can't drift, the
 * same pattern as ServiceStatusBadge. Rendered for every recognized type; a
 * legacy/unknown type shows the neutral "Service" fallback.
 */
export function ServiceTypeBadge({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  const key = deriveServiceType(service.type);
  return <Badge variant="outline">{t(SERVICE_TYPE_LABEL[key])}</Badge>;
}

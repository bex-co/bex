import { ReadUnavailable } from "@/common/components/read-unavailable";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ProjectResourceFamily } from "@/features/projects/lib/resource-filter";
import type { UnreadResources } from "@/features/environments/lib/unread-resources";

/**
 * Says why some of a card's rows may be missing instead of letting it read as
 * empty: a failed list offers a read-only Try again; a list still answering
 * shows a neutral loading line. `families` are the ones this card depends on.
 */
export function UnreadResourcesNotice({
  unread,
  families,
}: {
  unread: UnreadResources;
  families: readonly ProjectResourceFamily[];
}) {
  const { t } = useTranslations();
  if (families.some((family) => unread.failed.includes(family))) {
    return (
      <ReadUnavailable
        message={t("environments.resourcesUnavailable")}
        onRetry={unread.onRetry}
        retrying={unread.retrying}
      />
    );
  }
  if (families.length > 0) {
    return (
      <p className="text-sm text-muted-foreground" role="status">
        {t("environments.resourcesLoading")}
      </p>
    );
  }
  return null;
}

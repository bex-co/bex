import { ReadUnavailable } from "@/common/components/read-unavailable";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A plan region whose catalog read settled without plans — failed, or empty —
 * says so and offers to read it again in place, instead of looking like an
 * empty or forever-loading picker (w4/m218).
 */
export function PlanCatalogUnavailable({
  onRetry,
  retrying,
}: {
  onRetry: () => void;
  retrying: boolean;
}) {
  const { t } = useTranslations();
  return (
    <ReadUnavailable
      message={t("common.planCatalogUnavailable")}
      onRetry={onRetry}
      retrying={retrying}
    />
  );
}

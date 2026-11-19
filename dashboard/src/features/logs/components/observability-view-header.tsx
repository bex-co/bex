import type { ReactNode } from "react";
import { PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A project observability view's header (w3/m505), as Render's: the eyebrow
 * "Observability view" over the view's name. The pending skeleton renders the
 * same header with a placeholder for the name, so the two share geometry.
 */
export function ObservabilityViewHeader({ name }: { name: ReactNode }) {
  const { t } = useTranslations();
  return (
    <div className="space-y-1">
      <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
        {t("logs.observabilityViewEyebrow")}
      </p>
      <PageHeader title={name} />
    </div>
  );
}

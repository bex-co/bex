import { AlertCircle } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Rendered when bex-api reports ErrMetricsUnavailable (503: no backend wired
 * for this metric, e.g. BEX_PROM_URL unset) — an explicit callout, not an
 * empty chart, so "no source" and "no traffic yet" never look the same.
 */
export function MetricUnavailable({ height = 180 }: { height?: number }) {
  const { t } = useTranslations();

  return (
    <div
      className="flex flex-col items-center justify-center gap-2 rounded-md border border-dashed text-sm text-muted-foreground"
      style={{ height }}
    >
      <AlertCircle className="h-4 w-4" />
      <span>{t("metrics.sourceNotConfigured")}</span>
    </div>
  );
}

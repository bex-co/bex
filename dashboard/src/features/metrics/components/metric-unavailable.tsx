import { AlertCircle } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { chartFrameStyle } from "@/features/metrics/components/chart-layout";

/**
 * Rendered when bex-api reports ErrMetricsUnavailable (503: no backend wired
 * for this metric, e.g. BEX_PROM_URL unset) — an explicit callout, not an
 * empty chart, so "no source" and "no traffic yet" never look the same.
 * `message` overrides the copy for other explicit non-data states (e.g. the
 * bandwidth query-error state, w1/m50).
 */
export function MetricUnavailable({
  height,
  message,
}: {
  /** Overrides the chart's responsive ratio with a fixed height. */
  height?: number;
  message?: string;
}) {
  const { t } = useTranslations();

  return (
    <div
      className="flex w-full flex-col items-center justify-center gap-2 rounded-md border border-dashed text-sm text-muted-foreground"
      style={chartFrameStyle(height)}
    >
      <AlertCircle className="h-4 w-4" />
      <span>{message ?? t("metrics.sourceNotConfigured")}</span>
    </div>
  );
}

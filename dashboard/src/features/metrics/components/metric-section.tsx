import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";
import { useTranslations } from "@/common/hooks/use-translations";

interface MetricSectionProps {
  title: string;
  result: UseMetricsResult;
  /** Rendered beside the title: a control, the latest value, a limit label… */
  headerExtra?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * One chart section of a metrics card: the shared title row + the
 * source-unavailable branches, so every section on the page keeps identical
 * header typography and 503 handling. Two honest 503 states: no metrics backend
 * ("source not configured") and — for a host/path-filtered request read — no
 * durable log store ("host/path filters need the log store", w5/m58); the
 * filtered chart is never silently unfiltered.
 */
export function MetricSection({
  title,
  result,
  headerExtra,
  children,
}: MetricSectionProps) {
  const { t } = useTranslations();
  const body = result.storeUnavailable ? (
    <MetricUnavailable message={t("metrics.hostPathStoreUnavailable")} />
  ) : result.unavailable ? (
    <MetricUnavailable />
  ) : (
    children
  );
  return (
    <div>
      <div className="mb-2 flex items-center justify-between gap-2">
        <div className="text-sm font-medium text-foreground">{title}</div>
        {headerExtra}
      </div>
      {body}
    </div>
  );
}

import { MetricUnavailable } from "@/features/metrics/components/metric-unavailable";
import type { UseMetricsResult } from "@/features/metrics/hooks/use-metrics";

interface MetricSectionProps {
  title: string;
  result: UseMetricsResult;
  /** Rendered beside the title: a control, the latest value, a limit label… */
  headerExtra?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * One chart section of a metrics card: the shared title row + the
 * source-unavailable branch, so every section on the page keeps identical
 * header typography and 503 handling.
 */
export function MetricSection({
  title,
  result,
  headerExtra,
  children,
}: MetricSectionProps) {
  return (
    <div>
      <div className="mb-2 flex items-center justify-between gap-2">
        <div className="text-sm font-medium text-foreground">{title}</div>
        {headerExtra}
      </div>
      {result.unavailable ? <MetricUnavailable /> : children}
    </div>
  );
}

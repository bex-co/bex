import { TriangleAlert } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Rendered when a metrics query FAILS with something other than the recognized
 * "source not configured" / "log store unavailable" 503s — a genuine
 * GraphQL/network/404 error (w9/m86). Before, every such error left the hook
 * with empty series and the chart rendered its "No data in range" empty state,
 * indistinguishable from a genuinely empty window — which is what hid the
 * w5/m71 wrong-identifier bug for months. This is deliberately styled distinct
 * from {@link MetricUnavailable} (destructive, not muted-dashed) so error,
 * not-configured, and empty are three visibly different states.
 */
export function MetricError({
  height = 180,
  message,
}: {
  height?: number;
  message?: string;
}) {
  const { t } = useTranslations();

  return (
    <div
      role="alert"
      className="text-destructive border-destructive/50 flex flex-col items-center justify-center gap-2 rounded-md border text-sm"
      style={{ height }}
    >
      <TriangleAlert className="h-4 w-4" />
      <span>{message ?? t("metrics.chartError")}</span>
    </div>
  );
}

import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useLogPanelHeight,
  type LogPanelFill,
} from "@/features/logs/hooks/use-log-panel-height";

// Monospace-ish bar widths so the placeholder reads as log lines, not a
// generic block.
const BAR_WIDTHS = [
  "w-[62%]",
  "w-[48%]",
  "w-[74%]",
  "w-[40%]",
  "w-[68%]",
  "w-[55%]",
  "w-[71%]",
  "w-[44%]",
];

/**
 * A log-viewer-shaped loading placeholder (w9/m63 t003): a bordered, muted
 * panel of monospace-width skeleton rows at the viewer's own height, so the
 * Logs tab holds its space during the type-resolve and first-page phases
 * instead of flashing a blank body or a bare spinner.
 */
// Enough placeholder rows to fill the tallest panel; the frame clips them.
const ROWS = 24;

export function LogPanelSkeleton({
  fillViewport = false,
}: {
  /** As LogLineList's `fillViewport`: the panel sized to the viewport.
   *  `"flush"` (a deploy's log) has no stream status line under it. */
  fillViewport?: LogPanelFill;
}) {
  const { t } = useTranslations();
  const { ref, height } = useLogPanelHeight<HTMLDivElement>(fillViewport);
  // The loaded body: the panel at its fixed height, then the stream status
  // line under it. Eight rows of placeholder in a panel a third as tall made
  // the page grow by 366px when the first lines landed.
  return (
    <div role="status" aria-label={t("logs.loading")} className="space-y-2">
      <div
        ref={ref}
        className="@container/logs overflow-hidden rounded-md border bg-muted/30 p-3"
        style={{ height }}
      >
        {/* Clipped inside the padding, so the last row ends whole above the
            border rather than cut by it. */}
        <div className="h-full space-y-2 overflow-hidden">
          {/* As the loaded rows do, a narrow panel stacks each row's time over
            its message. */}
          {Array.from({ length: ROWS }, (_, i) => (
            <div
              key={i}
              className="flex items-center gap-3 @max-md/logs:flex-col @max-md/logs:items-start @max-md/logs:gap-1.5"
            >
              <Skeleton className="h-3 w-16 shrink-0" />
              <Skeleton
                className={`h-3 ${BAR_WIDTHS[i % BAR_WIDTHS.length]}`}
              />
            </div>
          ))}
        </div>
      </div>
      {fillViewport === "flush" ? null : <Skeleton className="h-4 w-44" />}
    </div>
  );
}

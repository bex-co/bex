import { cn } from "@/common/lib/utils/utils";

/** How close a meter is to its limit; picks the fill colour. */
export type UsageMeterLevel = "ok" | "warning" | "reached";

interface UsageMeterProps {
  /** The meter's accessible name. */
  label: string;
  /**
   * Usage as a share of the limit (clamped to [0, 1] for display), or null
   * when the reading is unavailable: the bar then stays empty and announces
   * `valueText` instead of a misleading 0%.
   */
  fraction: number | null;
  level: UsageMeterLevel;
  /** What a screen reader hears for the value, e.g. "$16.00 of $20.00". */
  valueText: string;
}

/**
 * A thin usage-against-limit progress bar (Billing's free instance hours,
 * Settings → Agents' model spend): primary below the warning point, amber
 * from it, destructive at the limit.
 */
export function UsageMeter({
  label,
  fraction,
  level,
  valueText,
}: UsageMeterProps) {
  const percent =
    fraction == null ? null : Math.min(100, Math.max(0, fraction * 100));
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={percent == null ? undefined : Math.round(percent)}
      aria-valuetext={valueText}
      data-state={percent == null ? "unavailable" : level}
      className="bg-muted h-2 w-full overflow-hidden rounded-full"
    >
      {percent == null ? null : (
        <div
          className={cn(
            "h-full",
            level === "reached"
              ? "bg-destructive"
              : level === "warning"
                ? "bg-amber-500"
                : "bg-primary",
          )}
          style={{ width: `${percent}%` }}
        />
      )}
    </div>
  );
}

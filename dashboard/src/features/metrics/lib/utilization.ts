import type { LimitSummary } from "@/features/metrics/lib/limit-summary";

/** Why a utilization (percentage-of-limit) chart draws no line. */
export type UtilizationEmptyReason =
  | "no-usage"
  | "no-limit"
  | "percentage-unavailable";

/**
 * Why a utilization chart has no line, or null when it has one. The Metrics
 * tab and Scaling's Recent Metrics both ask (w5/m125).
 *
 * The percentage read keeps each pod's own limit history, so its points draw
 * whenever any survived. The limit read is the current pods' only: it is empty
 * for a sleeping or suspended service as for a limitless one, so only a service
 * running instances with no limit is limitless. A parked one whose percentages
 * did not survive has them unavailable, not undefined (w4/192).
 */
export function utilizationEmptyReason({
  percentage,
  usage,
  limit,
  runsInstances,
}: {
  /** The percentage read has a point. */
  percentage: boolean;
  /** The absolute usage read has a point. */
  usage: boolean;
  /** The current pods' limits. */
  limit: LimitSummary;
  /** The service runs instances now: not suspended or asleep. */
  runsInstances: boolean;
}): UtilizationEmptyReason | null {
  if (percentage) return null;
  if (!usage) return "no-usage";
  if (limit.kind === "none" && runsInstances) return "no-limit";
  return "percentage-unavailable";
}

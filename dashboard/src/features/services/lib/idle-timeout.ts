// Shared auto-sleep eligibility policy plus idle-window helpers for Settings —
// a bex extension over Render (`spec.idleTTLSeconds`; ADR004, w1/m4.5).

import { isWebServiceType } from "@/features/services/lib/service-type";

/**
 * Preset windows offered in the Settings select, in seconds. 0 means the
 * platform default idle window, which the operator resolves to a real value
 * (15 min for free services, w6/m116) — distinct from the explicit "15 min"
 * preset in that it follows whatever the platform default is, rather than
 * pinning a number. Kept small and human — the point is "sleep quickly to save
 * money", not fine-grained tuning.
 */
export const IDLE_TIMEOUT_PRESETS = [0, 300, 900, 1800, 3600, 7200] as const;

/**
 * Whether an App on this plan auto-sleeps at all. Only the free tier sleeps
 * (paid tiers are always-on — w1/m4); an untiered bare-CR App (plan null)
 * defaults to free in the operator, so it sleeps too.
 */
export function planSleeps(plan: string | null): boolean {
  return plan === null || plan === "free";
}

/**
 * Whether this service has both sides of auto-sleep: a free plan that may be
 * parked and a public web route through the activator that can wake it again.
 * Private services serve HTTP only in-cluster, where no activator observes
 * traffic, so accepting an idle window for them would promise no wake path.
 */
export function autoSleepEligibleType(type: string): boolean {
  return isWebServiceType(type);
}

/** Whether both the service type and plan participate in auto-sleep. */
export function autoSleepEligible(type: string, plan: string | null): boolean {
  return autoSleepEligibleType(type) && planSleeps(plan);
}

/**
 * The select's options: the current value is always present (even if it's not a
 * preset — e.g. a value set via the API/CLI), so the control never silently
 * drops or misrepresents it. Sorted ascending, 0 (default) first.
 */
export function idleTimeoutOptions(current: number): number[] {
  const set = new Set<number>(IDLE_TIMEOUT_PRESETS);
  set.add(Math.max(0, current));
  return [...set].sort((a, b) => a - b);
}

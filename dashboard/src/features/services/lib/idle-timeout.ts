// Idle-timeout (auto-sleep window) helpers for the Settings control — a bex
// extension over Render (`spec.idleTTLSeconds`; docs/deployment.md, w1/m4.5).

/**
 * Preset windows offered in the Settings select, in seconds. 0 is the platform
 * default (the operator's own idle window). Kept small and human — the point is
 * "sleep quickly to save money", not fine-grained tuning.
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
 * The select's options: the current value is always present (even if it's not a
 * preset — e.g. a value set via the API/CLI), so the control never silently
 * drops or misrepresents it. Sorted ascending, 0 (default) first.
 */
export function idleTimeoutOptions(current: number): number[] {
  const set = new Set<number>(IDLE_TIMEOUT_PRESETS);
  set.add(Math.max(0, current));
  return [...set].sort((a, b) => a - b);
}

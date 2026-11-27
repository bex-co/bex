import type { HistoryState } from "@tanstack/history";

declare module "@tanstack/history" {
  interface HistoryState {
    /**
     * Manual sync's plan found no changes and saved no sync, so it hands its
     * outcome to `/blueprints/<id>/sync` in the navigation's state, as
     * Render's hands over its planned sync (w3/m678).
     */
    blueprintPlanNoChanges?: boolean;
  }
}

/** The navigation state Manual sync hands a plan that saved no sync. */
export const NO_CHANGES_PLAN_STATE: HistoryState = {
  blueprintPlanNoChanges: true,
};

/**
 * Whether the navigation handed over a plan with no changes, which `/sync`
 * draws as Resources already up to date. False when nothing was handed over
 * (a reload of another page's history entry, a pasted link).
 */
export function handedOverNoChangesPlan(
  state: HistoryState | undefined,
): boolean {
  return state?.blueprintPlanNoChanges === true;
}

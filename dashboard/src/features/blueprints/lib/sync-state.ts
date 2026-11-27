import type { BlueprintSyncAction } from "@/features/blueprints/types";
import type { en } from "@/i18n";

/**
 * A Blueprint sync's states, Render's (docs/render-artifacts/blueprint-pages.md
 * § Sync states): created (a plan waiting for approval), pending, running,
 * success and error. Only success and error are settled; a sync's page and
 * Syncs keep reading one that is not (w3/m678).
 */
export function isSettledSyncState(state: string): boolean {
  return state === "success" || state === "error";
}

/** Whether a sync is applying now: Render's card animates these. */
export function isApplyingSyncState(state: string): boolean {
  return state === "pending" || state === "running";
}

/** Each sync state's name: a state icon's accessible name, the applying badge. */
export const SYNC_STATE_LABEL: Record<string, keyof typeof en> = {
  created: "blueprints.stateCreated",
  pending: "blueprints.stateRunning",
  running: "blueprints.stateRunning",
  success: "blueprints.stateSuccess",
  error: "blueprints.statusError",
};

/**
 * The sentence each operation a sync changes something with reads as. A
 * plan's unchanged resources (noop) are not actions, as Render's plan has
 * none for them.
 */
export const SYNC_ACTION_SENTENCE: Record<string, keyof typeof en> = {
  create: "blueprints.syncActionCreate",
  update: "blueprints.syncActionUpdate",
  detach: "blueprints.syncActionDetach",
  error: "blueprints.syncActionError",
};

/** The actions a sync changes something with, in the order it planned them. */
export function changingSyncActions(
  actions: readonly BlueprintSyncAction[],
): BlueprintSyncAction[] {
  return actions.filter(({ action }) =>
    Object.hasOwn(SYNC_ACTION_SENTENCE, action.operation),
  );
}

// Shared allowed resource-action projection for unit tests that render
// service/deploy/datastore controls without a live Apollo client (w6/m143,
// w4/m578).
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";

const ACTIONS = [
  "suspend",
  "resume",
  "restart",
  "deploy",
  "cancel_deploy",
  "rollback",
  "cron_run_now",
  "cron_cancel_run",
];

/** Every action allowed, unless `overrides` names its outcome or the
 *  precondition that blocks it. */
export function mockAllowedResourceActions(
  resourceId = "app",
  overrides: Record<
    string,
    { outcome?: "allowed" | "denied"; precondition?: string }
  > = {},
) {
  const snapshot = toResourceSnapshot(
    "tea-test",
    resourceId,
    ACTIONS.map((action) => ({
      action,
      outcome: overrides[action]?.outcome ?? "allowed",
      reason: null,
      precondition: overrides[action]?.precondition ?? null,
    })),
  );
  const state = {
    status: "ready" as const,
    snapshot,
    refresh: async () => undefined,
  };
  return {
    useServerActions: () => state,
    useDeployActions: () => state,
    useDatastoreActions: () => state,
  };
}

/**
 * Who is asking a datastore's Suspend and Resume (w4/m578): a caller they are
 * allowed to, a Developer in a protected environment, or a Viewer.
 */
export type LifecycleCaller = "allowed" | "adminOnly" | "viewer";

const LIFECYCLE_ANSWER: Record<
  LifecycleCaller,
  { outcome: string; reason: string | null }
> = {
  allowed: { outcome: "allowed", reason: null },
  adminOnly: {
    outcome: "denied",
    reason: "protected_environment_admin_required",
  },
  viewer: { outcome: "denied", reason: "insufficient_permission" },
};

/** The suspend and resume rows a datastore projection answers `caller`. */
export function lifecycleRows(caller: LifecycleCaller) {
  return (["suspend", "resume"] as const).map((action) => ({
    action,
    ...LIFECYCLE_ANSWER[caller],
    precondition: null,
  }));
}

/** A ready projection state for `resourceId` in tea-test. */
export function lifecycleState(resourceId: string, caller: LifecycleCaller) {
  return {
    status: "ready" as const,
    snapshot: toResourceSnapshot("tea-test", resourceId, lifecycleRows(caller)),
    refresh: async () => undefined,
  };
}

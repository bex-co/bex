// Shared allowed resource-action projection for unit tests that render
// service/deploy controls without a live Apollo client (w6/m143).
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
  };
}

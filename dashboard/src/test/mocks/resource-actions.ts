// Shared allowed resource-action projection for unit tests that render
// service/deploy controls without a live Apollo client (w6/m143).
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";

export function mockAllowedResourceActions(resourceId = "app") {
  const snapshot = toResourceSnapshot("tea-test", resourceId, [
    { action: "suspend", outcome: "allowed", reason: null, precondition: null },
    { action: "resume", outcome: "allowed", reason: null, precondition: null },
    { action: "restart", outcome: "allowed", reason: null, precondition: null },
    { action: "deploy", outcome: "allowed", reason: null, precondition: null },
    {
      action: "cancel_deploy",
      outcome: "allowed",
      reason: null,
      precondition: null,
    },
    { action: "rollback", outcome: "allowed", reason: null, precondition: null },
  ]);
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

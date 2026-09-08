import { useRef } from "react";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { useAuth } from "@/features/auth/auth-provider";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import { accessMessage } from "@/features/capabilities/capability-policy";
import type { MobileSafeActionId } from "./registry";

/** Bind a device confirmation to the identity and access that displayed it. */
export function useActionAccess() {
  const { state } = useAuth();
  const { selected } = useWorkspace();
  const capabilities = useCapabilities();
  const key = JSON.stringify([
    state.status === "signedIn" ? state.session.sessionId : null,
    selected?.id ?? null,
    capabilities.generation,
  ]);
  const latest = useRef({ key, capabilities });
  latest.current = { key, capabilities };
  return {
    key,
    reason: (action: MobileSafeActionId) =>
      accessMessage(
        capabilities.state,
        capabilities.offline,
        capabilities.denied("can_operate") ||
          ((action === "rollback-service" ||
            action === "create-agent-session") &&
            capabilities.denied("can_create")),
      ),
    isCurrent: (binding: string, action: MobileSafeActionId) => {
      const current = latest.current;
      return (
        binding === current.key &&
        current.capabilities.generation === dataBoundary.getGeneration() &&
        current.capabilities.allows("can_operate") &&
        ((action !== "rollback-service" && action !== "create-agent-session") ||
          current.capabilities.allows("can_create"))
      );
    },
  };
}

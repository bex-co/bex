import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";
import {
  composerSpendGate,
  type ComposerSpendGate,
} from "@/features/agent-environment/lib/spend";

/**
 * The workspace's agent-model spend gate for the new-session composer
 * (ADR091 D6): nothing for a BYO-key workspace or below 80% of the cap.
 */
export function useComposerSpendGate(): ComposerSpendGate {
  const { currentWorkspaceId } = useWorkspace();
  const { environment } = useAgentEnvironment(currentWorkspaceId ?? null);
  return composerSpendGate(environment);
}

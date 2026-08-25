import { useWorkspace } from "@/features/workspaces/context";
import { isAgentsDashboardEnabled } from "@/config/growthbook";

/** Client hook for the dashboard `/agents` GrowthBook flag. */
export function useAgentsFeatureEnabled(): boolean {
  const { currentWorkspaceId } = useWorkspace();
  return isAgentsDashboardEnabled(currentWorkspaceId);
}

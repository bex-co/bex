import { redirect } from "@tanstack/react-router";
import { isAgentsDashboardEnabled } from "@/config/growthbook";
import type { RouterContext } from "@/common/types/router-context";

type AgentsFeatureContext = Pick<RouterContext, "workspaceId">;

/** Route guard: `/agents` is enabled only for GrowthBook-targeted workspaces. */
export const requireAgentsFeature = () => {
  return ({ context }: { context: AgentsFeatureContext }): void => {
    if (!isAgentsDashboardEnabled(context.workspaceId)) {
      throw redirect({ to: "/" });
    }
  };
};

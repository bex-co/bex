import { redirect } from "@tanstack/react-router";
import { isRouterDashboardEnabled } from "@/config/growthbook";
import type { RouterContext } from "@/common/types/router-context";

type RouterFeatureContext = Pick<RouterContext, "workspaceId">;

/** Route guard: `/router` is enabled only for GrowthBook-targeted workspaces. */
export const requireRouterFeature = () => {
  return ({ context }: { context: RouterFeatureContext }): void => {
    if (!isRouterDashboardEnabled(context.workspaceId)) {
      throw redirect({ to: "/" });
    }
  };
};

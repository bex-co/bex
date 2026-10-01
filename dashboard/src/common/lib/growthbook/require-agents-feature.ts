import { redirect } from "@tanstack/react-router";
import { isAgentsDashboardEnabled } from "@/config/growthbook";
import type { RouterContext } from "@/common/types/router-context";
import { WorkspacesDocument } from "@/graphql/definitions";
import { translatedText } from "@/common/lib/document-head";

type AgentsFeatureContext = Pick<RouterContext, "workspaceId" | "client">;

/** Route guard: `/agents` is enabled only for GrowthBook-targeted workspaces. */
export const requireAgentsFeature = () => {
  return async ({ context }: { context: AgentsFeatureContext }) => {
    const { data } = await context.client.query({
      query: WorkspacesDocument,
      fetchPolicy: "cache-first",
      errorPolicy: "none",
    });
    if (!data?.workspaces) {
      // An unavailable membership response must not poison explicit retries.
      context.client.cache.evict({ id: "ROOT_QUERY", fieldName: "workspaces" });
      throw new Error(translatedText("common.resourceErrorBody"));
    }
    const workspaces = data.workspaces.filter((workspace) => !!workspace?.id);
    const workspaceId = workspaces.some((w) => w?.id === context.workspaceId)
      ? context.workspaceId
      : workspaces[0]?.id;
    if (!isAgentsDashboardEnabled(workspaceId)) {
      throw redirect({ to: "/" });
    }
    return { workspaceId };
  };
};

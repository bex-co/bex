import { RouterAvailableDocument } from "@/graphql/definitions";
import { createFileRoute, Navigate, redirect } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireRouterFeature } from "@/common/lib/growthbook/require-router-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { isRouterDashboardEnabled } from "@/config/growthbook";
import { useWorkspace } from "@/features/workspaces/context";
import { RouterPage, RouterPageSkeleton } from "@/features/router/page";

export const Route = createFileRoute("/router")({
  staticData: { chrome: true },
  beforeLoad: async ({ context, location }) => {
    requireAuth()({ context, location });
    requireRouterFeature()({ context });
    const { data } = await context.client.query({
      query: RouterAvailableDocument,
      variables: { ownerId: context.workspaceId! },
      fetchPolicy: "network-only",
    });
    if (!data?.routerAvailable) throw redirect({ to: "/" });
  },
  component: RouterRoute,
  pendingComponent: RouterPageSkeleton,
  head: ({ match }) => translatedTitleHead("router.title", match),
});

function RouterRoute() {
  const { currentWorkspaceId } = useWorkspace();
  const enabled = isRouterDashboardEnabled(currentWorkspaceId);
  return enabled ? <RouterPage /> : <Navigate to="/" replace />;
}

import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import { DeployRollbackSkeleton } from "@/common/components/route-skeletons";
import { prefetchRollbackPage } from "@/features/deploys/lib/deploy-page-prefetch";

/**
 * A deploy's Confirm Rollback page (w1/m646): Render's
 * `/<type>/srv-…/deploys/dep-…/rollback`, the deploy page with the
 * configuration the rollback changes in place of its log
 * (docs/render-artifacts/rollback-comparison.md). Un-nested from the deploy
 * route, so it renders into the `static.$serviceId` layout itself.
 */
export const Route = createFileRoute(
  "/static/$serviceId/deploys/$deployId_/rollback",
)({
  component: RouteComponent,
  pendingComponent: DeployRollbackSkeleton,
  // The page mounts with the deploy, its Notices (w1/m659) and the rollback
  // comparison (w1/m660) read.
  loader: ({ context, params, cause }) =>
    prefetchRollbackPage(
      context.client,
      params.serviceId,
      params.deployId,
      cause,
    ),
});

function RouteComponent() {
  const { serviceId, deployId } = Route.useParams();
  return (
    <DeployDetailPage serviceId={serviceId} deployId={deployId} rollback />
  );
}

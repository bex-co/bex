import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";
import { DeployRollbackSkeleton } from "@/common/components/route-skeletons";

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
});

function RouteComponent() {
  const { serviceId, deployId } = Route.useParams();
  return (
    <DeployDetailPage serviceId={serviceId} deployId={deployId} rollback />
  );
}

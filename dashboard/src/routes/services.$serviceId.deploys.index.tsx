import { createFileRoute } from "@tanstack/react-router";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import { DeploysListSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/deploys/")({
  component: RouteComponent,
  pendingComponent: DeploysListSkeleton,
});

// The dedicated Deploys tab (w9/002): Render's standalone deploy-history
// list, distinct from the Events feed. Nests under the `services.$serviceId`
// layout route like every other tab; its rows link into the sibling
// per-deploy route ($deployId).
function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <DeploysListPage serviceId={serviceId} />;
}

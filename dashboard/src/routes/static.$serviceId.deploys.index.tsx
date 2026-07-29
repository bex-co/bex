import { createFileRoute } from "@tanstack/react-router";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";

/**
 * A static_site has no Deploys tab in its sidebar (Render parity, w5/m57), but
 * its deploy history stays reachable under /static so the Events feed's deploy
 * links and the deploy-detail back button resolve within the /static tree
 * (loop-free) instead of bouncing to /services.
 */
export const Route = createFileRoute("/static/$serviceId/deploys/")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <DeploysListPage serviceId={serviceId} />;
}

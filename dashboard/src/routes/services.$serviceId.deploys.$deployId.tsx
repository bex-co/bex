import { createFileRoute } from "@tanstack/react-router";
import { DeployDetailPage } from "@/features/deploys/components/deploy-detail-page";

export const Route = createFileRoute(
  "/services/$serviceId/deploys/$deployId",
)({
  component: RouteComponent,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Deploy · bex dashboard` }],
  }),
});

// The per-deploy page (w9/m1): Render's `/web/srv-…/deploys/dep-…` twin.
// Nests under the `services.$serviceId` layout route, so the service header +
// tab nav stay in place — only the deploy-specific header + log viewer render
// into the shared `<Outlet/>`.
function RouteComponent() {
  const { serviceId, deployId } = Route.useParams();
  return <DeployDetailPage serviceId={serviceId} deployId={deployId} />;
}

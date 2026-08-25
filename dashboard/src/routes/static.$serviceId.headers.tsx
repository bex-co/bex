import { createFileRoute } from "@tanstack/react-router";
import { ServiceHeadersPage } from "./services.$serviceId.headers";
import { StaticEdgeRulesSkeleton } from "@/common/components/route-skeletons";

/** Static-site Headers tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/headers")({
  component: RouteComponent,
  pendingComponent: StaticEdgeRulesSkeleton,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceHeadersPage serviceId={serviceId} />;
}

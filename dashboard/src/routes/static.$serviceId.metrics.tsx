import { createFileRoute } from "@tanstack/react-router";
import { ServiceMetricsPage } from "./services.$serviceId.metrics";

/** Static-site Metrics tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/metrics")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceMetricsPage serviceId={serviceId} />;
}

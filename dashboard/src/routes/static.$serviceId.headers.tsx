import { createFileRoute } from "@tanstack/react-router";
import { ServiceHeadersPage } from "./services.$serviceId.headers";

/** Static-site Headers tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/headers")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceHeadersPage serviceId={serviceId} />;
}

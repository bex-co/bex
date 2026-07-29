import { createFileRoute } from "@tanstack/react-router";
import { ServiceEventsPage } from "./services.$serviceId.events";

/** Static-site Events tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/events")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceEventsPage serviceId={serviceId} />;
}

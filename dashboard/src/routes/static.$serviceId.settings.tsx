import { createFileRoute } from "@tanstack/react-router";
import { ServiceSettingsPage } from "./services.$serviceId.settings";

/** Static-site Settings tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/settings")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceSettingsPage serviceId={serviceId} />;
}

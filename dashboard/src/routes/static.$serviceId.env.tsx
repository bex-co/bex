import { createFileRoute } from "@tanstack/react-router";
import { ServiceEnvPage } from "./services.$serviceId.env";

/** Static-site Environment tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/env")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceEnvPage serviceId={serviceId} />;
}

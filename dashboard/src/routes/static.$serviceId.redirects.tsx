import { createFileRoute } from "@tanstack/react-router";
import { ServiceRedirectsPage } from "./services.$serviceId.redirects";

/** Static-site Redirects/Rewrites tab — shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/redirects")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceRedirectsPage serviceId={serviceId} />;
}

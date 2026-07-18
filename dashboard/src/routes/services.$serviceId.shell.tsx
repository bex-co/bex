import { createFileRoute } from "@tanstack/react-router";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";

export const Route = createFileRoute("/services/$serviceId/shell")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceShellPage serviceId={serviceId} />;
}

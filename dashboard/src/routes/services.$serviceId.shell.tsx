import { createFileRoute } from "@tanstack/react-router";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";

export const Route = createFileRoute("/services/$serviceId/shell")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceShellPage serviceId={serviceId} />
    </NonStaticRoute>
  );
}

import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";
import { ServiceShellSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/shell")({
  head: serviceTabHead("services.navShell", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceShellSkeleton,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceShellPage serviceId={serviceId} />
    </NonStaticRoute>
  );
}

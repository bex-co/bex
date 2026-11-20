import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";
import { ServiceShellSkeleton } from "@/common/components/route-skeletons";
import { str } from "@/features/logs/lib/log-search";

export const Route = createFileRoute("/services/$serviceId/shell")({
  head: serviceTabHead("services.navShell", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceShellSkeleton,
  // The instance a log line's SSH into instance opens the Shell on (w3/m528,
  // Render's `/shell/{instance}`).
  validateSearch: (search: Record<string, unknown>): { instance?: string } => ({
    instance: str(search.instance),
  }),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const { instance } = Route.useSearch();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceShellPage serviceId={serviceId} instance={instance} />
    </NonStaticRoute>
  );
}

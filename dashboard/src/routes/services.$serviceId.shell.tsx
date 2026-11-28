import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute, getRouteApi } from "@tanstack/react-router";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";
import { ServiceShellSkeleton } from "@/common/components/route-skeletons";
import { str } from "@/features/logs/lib/log-search";
import { shellAccess } from "@/features/services/lib/ssh-remedy";

export const Route = createFileRoute("/services/$serviceId/shell")({
  head: serviceTabHead("services.navShell", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceShellPending,
  // The instance a log line's SSH into instance opens the Shell on (w3/m528,
  // Render's `/shell/{instance}`).
  validateSearch: (search: Record<string, unknown>): { instance?: string } => ({
    instance: str(search.instance),
  }),
});

// The frame draws what the page will for the service the parent loaded: the
// terminal and SSH command, or the card a service without a shell lands on
// (w3/124).
function ServiceShellPending() {
  const parent = getRouteApi("/services/$serviceId").useLoaderData();
  return (
    <ServiceShellSkeleton
      access={parent?.state === "ready" ? shellAccess(parent.resource) : null}
    />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const { instance } = Route.useSearch();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceShellPage serviceId={serviceId} instance={instance} />
    </NonStaticRoute>
  );
}

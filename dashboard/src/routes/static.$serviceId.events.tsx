import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { ServiceEventsPage } from "./services.$serviceId.events";
import { parseEventsSearch } from "@/features/events/service-event-catalog";
import { ServiceEventsSkeleton } from "@/common/components/route-skeletons";

/** Static-site Events tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/events")({
  head: serviceTabHead("services.navEvents", "/static/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceEventsSkeleton,
  validateSearch: parseEventsSearch,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const { et } = Route.useSearch();
  return <ServiceEventsPage serviceId={serviceId} eventTypes={et} />;
}

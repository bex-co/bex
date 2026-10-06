import { createFileRoute, getRouteApi } from "@tanstack/react-router";
import { ServiceSettingsPage } from "./services.$serviceId.settings";
import { ServiceSettingsSkeleton } from "@/common/components/route-skeletons";
import { toServiceView } from "@/features/services/lib/status";

/** Static-site Settings tab — the shared page under the /static base (w5/m57). */
export const Route = createFileRoute("/static/$serviceId/settings")({
  component: RouteComponent,
  pendingComponent: StaticSettingsPending,
});

function StaticSettingsPending() {
  const parent = getRouteApi("/static/$serviceId").useLoaderData();
  return (
    <ServiceSettingsSkeleton
      service={
        parent?.state === "ready" ? toServiceView(parent.resource) : undefined
      }
      staticSite
    />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceSettingsPage serviceId={serviceId} />;
}

import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute, getRouteApi } from "@tanstack/react-router";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { ServiceComputePage } from "@/features/services/components/service-compute-page";
import { ServiceComputeSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/compute")({
  head: serviceTabHead("services.navCompute", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceComputePending,
});

function ServiceComputePending() {
  const parent = getRouteApi("/services/$serviceId").useLoaderData();
  const ready = parent?.state === "ready" ? parent.resource : null;
  return (
    <ServiceComputeSkeleton
      serviceType={ready ? ready.type : null}
      currentPlan={ready ? (ready.plan ?? null) : undefined}
    />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceComputePage serviceId={serviceId} />
    </NonStaticRoute>
  );
}

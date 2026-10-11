import { createFileRoute, getRouteApi } from "@tanstack/react-router";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import { useCachedServiceType } from "@/features/services/hooks/use-cached-service-type";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { useServer } from "@/features/services/hooks/use-server";
import { ServicePlanSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/plan")({
  component: ServicePlanPage,
  pendingComponent: ServicePlanPending,
});

function ServicePlanPending() {
  const parent = getRouteApi("/services/$serviceId").useLoaderData();
  return (
    <ServicePlanSkeleton
      serviceType={parent?.state === "ready" ? parent.resource.type : null}
    />
  );
}

/** Mounted only while the server read is missing; the list cache may know the type. */
function CachedServicePlanSkeleton({ serviceId }: { serviceId: string }) {
  return <ServicePlanSkeleton serviceType={useCachedServiceType(serviceId)} />;
}

/**
 * The plan-picker page (w5/m7), reached from the Settings tab's Instance
 * Type "Update" link — mirrors Render's own URL split (a service's plan page
 * is a sibling route of its settings page, not nested under it).
 *
 * Gated on the server query the same way the Settings tab is: the picker
 * seeds its selection from `currentPlan` only on mount, so mounting it before
 * the query resolves would freeze the pre-selection at the not-yet-loaded
 * null value.
 */
function ServicePlanPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId, { poll: false });
  if (!service && loading) {
    return <CachedServicePlanSkeleton serviceId={serviceId} />;
  }

  return (
    <NonStaticRoute serviceId={serviceId}>
      <InstanceTypePicker
        serviceId={serviceId}
        currentPlan={service?.plan ?? null}
        serviceType={service?.type ?? null}
      />
    </NonStaticRoute>
  );
}

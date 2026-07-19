import { createFileRoute } from "@tanstack/react-router";
import { Skeleton } from "@/common/components/ui/skeleton";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { useServer } from "@/features/services/hooks/use-server";

export const Route = createFileRoute("/services/$serviceId/plan")({
  component: ServicePlanPage,
});

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
export function ServicePlanPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId);

  if (!service && loading) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <NonStaticRoute serviceId={serviceId}>
      <InstanceTypePicker
        serviceId={serviceId}
        currentPlan={service?.plan ?? null}
      />
    </NonStaticRoute>
  );
}

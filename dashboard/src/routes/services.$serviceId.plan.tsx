import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { PlanPickerGridSkeleton } from "@/common/components/detail-skeletons";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { useServer } from "@/features/services/hooks/use-server";
import { ServicePlanSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/plan")({
  component: ServicePlanPage,
  pendingComponent: ServicePlanSkeleton,
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
function ServicePlanPage() {
  const { serviceId } = Route.useParams();
  const { service, loading } = useServer(serviceId, { poll: false });

  if (!service && loading) {
    // Mirror InstanceTypePicker's outer shape — titled card, tier grid, footer
    // actions — so the pre-query load reads as the picker it becomes.
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-48" />
        </CardHeader>
        <CardContent className="space-y-6">
          <PlanPickerGridSkeleton />
          <div className="flex justify-end gap-2 border-t pt-4">
            <Skeleton className="h-9 w-20" />
            <Skeleton className="h-9 w-20" />
          </div>
        </CardContent>
      </Card>
    );
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

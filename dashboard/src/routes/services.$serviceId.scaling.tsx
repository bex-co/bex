import { createFileRoute } from "@tanstack/react-router";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { AutoscalingSection } from "@/features/services/components/autoscaling-section";
import { ManualScalingSection } from "@/features/services/components/manual-scaling-section";
import { ScalingRecentMetrics } from "@/features/services/components/scaling-recent-metrics";
import { useAutoscaling } from "@/features/services/hooks/use-autoscaling";
import { useServer } from "@/features/services/hooks/use-server";
import { isCron, isStaticSite } from "@/features/services/lib/service-type";

export const Route = createFileRoute("/services/$serviceId/scaling")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <NonStaticRoute serviceId={serviceId}>
      <ServiceScalingPage serviceId={serviceId} />
    </NonStaticRoute>
  );
}

/**
 * The Scaling tab, mirroring Render's live page structure (w7/m43):
 * Autoscaling card ⊕ Manual Scaling card (mutually exclusive — the manual
 * card shows exactly while autoscaling is off), then Recent Metrics (48h
 * utilization + instances) under both. Exported taking `serviceId` as a prop
 * so a routing test can mount it without the file Route's param context.
 */
export function ServiceScalingPage({ serviceId }: { serviceId: string }) {
  const { service } = useServer(serviceId);
  // One hook instance for the page: the card renders from it and the manual
  // card's exclusion gate reads it, so `saving`/`enabled` can never disagree.
  const autoscaling = useAutoscaling(serviceId);

  // Only service types with a replica concept scale manually — cron jobs have
  // no long-running pods and static sites serve from the object store (the
  // same gating the Settings page applies to its scaling-adjacent rows).
  const scalable =
    service != null && !isCron(service) && !isStaticSite(service);

  return (
    <div className="space-y-6">
      <AutoscalingSection autoscaling={autoscaling} />
      {/* Reserve the manual-card slot while autoscaling state resolves so the
          card doesn't pop in and shift the layout (w9/m63 t003). */}
      {scalable && autoscaling.loading && <CardSkeleton rows={2} />}
      {scalable && !autoscaling.loading && !autoscaling.enabled && (
        <ManualScalingSection
          serviceId={serviceId}
          replicas={service.replicas ?? 1}
        />
      )}
      <ScalingRecentMetrics serviceId={serviceId} />
    </div>
  );
}

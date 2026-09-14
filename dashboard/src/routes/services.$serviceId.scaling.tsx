import { createFileRoute } from "@tanstack/react-router";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { AutoscalingSection } from "@/features/services/components/autoscaling-section";
import { ManualScalingSection } from "@/features/services/components/manual-scaling-section";
import { ScalingRecentMetrics } from "@/features/services/components/scaling-recent-metrics";
import { useAutoscaling } from "@/features/services/hooks/use-autoscaling";
import { useServer } from "@/features/services/hooks/use-server";
import { supportsScaling } from "@/features/services/lib/service-type";
import { ServiceScalingSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/scaling")({
  component: RouteComponent,
  pendingComponent: ServiceScalingSkeleton,
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
 *
 * Gated to web / private / background_worker (m101/t002). Cron (and any other
 * ineligible type that reaches this URL) gets an explanation card — Disk's nav
 * omission plus Shell's honest URL-reachability pattern. While `service` is
 * still null, no autoscaling card is rendered (no flash for a cron job).
 */
export function ServiceScalingPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { service } = useServer(serviceId, { poll: false });
  // One hook instance for the page: the card renders from it and the manual
  // card's exclusion gate reads it, so `saving`/`enabled` can never disagree.
  const autoscaling = useAutoscaling(serviceId);

  const scalable = service != null && supportsScaling(service);
  const ineligible = service != null && !supportsScaling(service);

  if (ineligible) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("services.scalingUnavailableTitle")}</CardTitle>
          <CardDescription>
            {t("services.scalingUnavailableBody")}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {scalable ? <AutoscalingSection autoscaling={autoscaling} /> : null}
      {/* Reserve the manual-card slot while autoscaling state resolves so the
          card doesn't pop in and shift the layout (w9/m63 t003). */}
      {scalable && autoscaling.loading ? <CardSkeleton rows={2} /> : null}
      {scalable && !autoscaling.loading && !autoscaling.enabled ? (
        <ManualScalingSection
          serviceId={serviceId}
          replicas={service.replicas ?? 1}
          plan={service.plan}
        />
      ) : null}
      {scalable ? <ScalingRecentMetrics serviceId={serviceId} /> : null}
    </div>
  );
}

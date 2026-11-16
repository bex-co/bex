import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import { AutoscalingSection } from "@/features/services/components/autoscaling-section";
import {
  ManualScalingSection,
  ManualScalingSkeleton,
} from "@/features/services/components/manual-scaling-section";
import { ScalingRecentMetrics } from "@/features/services/components/scaling-recent-metrics";
import { useAutoscaling } from "@/features/services/hooks/use-autoscaling";
import { useServer } from "@/features/services/hooks/use-server";
import { supportsScaling } from "@/features/services/lib/service-type";
import { runsInstances } from "@/features/services/lib/status";
import { ServiceScalingSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/scaling")({
  head: serviceTabHead("services.navScaling", "/services/$serviceId"),
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
      {scalable ? (
        <AutoscalingSection autoscaling={autoscaling} plan={service.plan} />
      ) : null}
      {/* Reserve the manual-card slot at that card's own geometry while
          autoscaling state resolves, so nothing shifts when it settles
          (w9/m63 t003, w4/688). */}
      {scalable && autoscaling.loading ? (
        <ManualScalingSkeleton plan={service.plan} />
      ) : null}
      {scalable &&
      !autoscaling.loading &&
      (!autoscaling.enabled || service.plan === "free") ? (
        <ManualScalingSection
          serviceId={serviceId}
          replicas={service.replicas ?? 1}
          plan={service.plan}
        />
      ) : null}
      {scalable ? (
        <ScalingRecentMetrics
          serviceId={serviceId}
          runsInstances={runsInstances(service)}
        />
      ) : null}
    </div>
  );
}

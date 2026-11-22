import { useEffect, useRef, type ReactNode } from "react";
import { useLocation } from "@tanstack/react-router";
import { ComputePlanSection } from "@/features/services/components/compute-plan-section";
import { AutoscalingSection } from "@/features/services/components/autoscaling-section";
import { ManualScalingSection } from "@/features/services/components/manual-scaling-section";
import { ManualScalingSkeleton } from "@/features/services/components/scaling-frames";
import { ScalingRecentMetrics } from "@/features/services/components/scaling-recent-metrics";
import { useAutoscaling } from "@/features/services/hooks/use-autoscaling";
import { useCachedServiceType } from "@/features/services/hooks/use-cached-service-type";
import { useServer } from "@/features/services/hooks/use-server";
import { useWorkspace } from "@/features/workspaces/context";
import { supportsScaling } from "@/features/services/lib/service-type";
import { hasServed, runsInstances } from "@/features/services/lib/status";
import { MANUAL_SCALING_ANCHOR } from "@/features/services/lib/compute-page";
import { ServiceComputeSkeleton } from "@/common/components/route-skeletons";

/** Mounted only while the server read is missing; the list cache may know the type. */
function CachedServiceComputeSkeleton({ serviceId }: { serviceId: string }) {
  return (
    <ServiceComputeSkeleton serviceType={useCachedServiceType(serviceId)} />
  );
}

/**
 * A service's Compute page, as Render's dashboard has had since 2026-08-26
 * (w6/m483, docs/render-artifacts/compute-page.md): the Compute section (the
 * compute plan, with Edit), then — for web, private and background worker
 * services — Render's scaling sections as its live Scaling page drew them
 * (w7/m43): Autoscaling ⊕ Manual Scaling (mutually exclusive — the manual
 * section shows exactly while autoscaling is off), then Recent Metrics. A cron
 * job has no Compute page: its URL lands on its Settings' Compute section, the
 * same ComputePlanSection (w3/m554).
 *
 * Gated on the server query: the plan editor seeds its selection from the
 * current plan, and a type-shaped skeleton holds the frame until it resolves.
 * Exported taking `serviceId` as a prop so a routing test can mount it without
 * the file Route's param context.
 */
export function ServiceComputePage({ serviceId }: { serviceId: string }) {
  const { service, loading } = useServer(serviceId, { poll: false });
  const scalable = service != null && supportsScaling(service);
  // One hook instance for the page: the card renders from it and the manual
  // card's exclusion gate reads it, so `saving`/`enabled` can never disagree.
  // It starts with the service read and stops once the type rules scaling out.
  const autoscaling = useAutoscaling(serviceId, {
    skip: service != null && !scalable,
  });
  const { currentWorkspace } = useWorkspace();

  if (!service && loading) {
    return <CachedServiceComputeSkeleton serviceId={serviceId} />;
  }

  return (
    <div className="space-y-6">
      <ComputePlanSection
        serviceId={serviceId}
        currentPlan={service?.plan ?? null}
        serviceType={service?.type ?? null}
        live={service == null || hasServed(service)}
      />
      {scalable ? (
        <>
          <AutoscalingSection
            autoscaling={autoscaling}
            plan={service.plan}
            workspacePlan={currentWorkspace?.plan}
          />
          <ManualScalingSlot>
            {/* Reserve the manual card's slot at its own geometry while
                autoscaling state resolves, so nothing shifts when it settles
                (w9/m63 t003, w4/688). */}
            {autoscaling.loading ? (
              <ManualScalingSkeleton plan={service.plan} />
            ) : !autoscaling.enabled || service.plan === "free" ? (
              <ManualScalingSection
                serviceId={serviceId}
                replicas={service.replicas ?? 1}
                plan={service.plan}
              />
            ) : null}
          </ManualScalingSlot>
          <ScalingRecentMetrics
            serviceId={serviceId}
            runsInstances={runsInstances(service)}
          />
        </>
      ) : null}
    </div>
  );
}

/**
 * The Manual Scaling section's anchor (`#manual-scaling`, where the old
 * Scaling URL redirects). It draws nothing while autoscaling hides the card,
 * and scrolls itself into view once when the URL names it, because the
 * section mounts after the service read the router's own hash scroll ran
 * before.
 */
function ManualScalingSlot({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const scrolled = useRef(false);
  const named = useLocation({
    select: (location) => location.hash === MANUAL_SCALING_ANCHOR,
  });
  const shown = children != null;
  useEffect(() => {
    if (shown && named && !scrolled.current) {
      scrolled.current = true;
      ref.current?.scrollIntoView();
    }
  }, [shown, named]);
  if (!shown) return null;
  return (
    <div ref={ref} id={MANUAL_SCALING_ANCHOR} className="scroll-mt-6">
      {children}
    </div>
  );
}

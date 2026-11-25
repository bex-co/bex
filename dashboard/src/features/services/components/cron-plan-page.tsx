import { useId } from "react";
import { useNavigate } from "@tanstack/react-router";
import { ComputePlanSectionFrame } from "@/features/services/components/compute-plan-frames";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import { useServer } from "@/features/services/hooks/use-server";
import { legacyFreeNotice } from "@/features/services/lib/instance-type";
import { hasServed } from "@/features/services/lib/status";
import { ServiceCronPlanSkeleton } from "@/common/components/route-skeletons";

/**
 * A cron job's plan page, Render's `/cron/<id>/plan` (w3/105,
 * docs/render-artifacts/cron-dashboard.md): its plan cards, open and ready to
 * change. Cancel returns to Settings. A change opens the deploy it started, as
 * Render's opens the new build, or returns to Settings when it started none.
 * Nothing links here, as on Render: the plan badge and the upgrade dialog go to
 * Settings' Compute section, the same cards behind an Edit button.
 *
 * Exported taking `serviceId` as a prop so a test can mount it without the
 * file Route's param context.
 */
export function CronPlanPage({ serviceId }: { serviceId: string }) {
  const { service, loading } = useServer(serviceId, { poll: false });
  const navigate = useNavigate();
  const noticeId = useId();

  if (!service && loading) {
    return <ServiceCronPlanSkeleton />;
  }

  const serviceType = service?.type ?? null;
  const currentPlan = service?.plan ?? null;
  const toSettings = () =>
    void navigate({
      to: "/services/$serviceId/settings",
      params: { serviceId },
    });

  return (
    <ComputePlanSectionFrame
      serviceType={serviceType}
      currentPlan={currentPlan}
      noticeId={noticeId}
    >
      <InstanceTypePicker
        serviceId={serviceId}
        currentPlan={currentPlan}
        serviceType={serviceType}
        live={service == null || hasServed(service)}
        onDone={toSettings}
        onSaved={(deployId) => {
          if (!deployId) return toSettings();
          void navigate({
            to: "/services/$serviceId/deploys/$deployId",
            params: { serviceId, deployId },
          });
        }}
        describedBy={
          legacyFreeNotice(serviceType, currentPlan) !== null
            ? noticeId
            : undefined
        }
      />
    </ComputePlanSectionFrame>
  );
}

import { useId, useState } from "react";
import { Pencil } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { InstanceTypePicker } from "@/features/services/components/instance-type-picker";
import {
  ComputePlanRowFrame,
  ComputePlanSectionFrame,
} from "@/features/services/components/compute-plan-frames";
import { ComputePlanSummary } from "@/features/services/components/instance-type-row";
import { legacyFreeNotice } from "@/features/services/lib/instance-type";

export interface ComputePlanSectionProps {
  serviceId: string;
  /** The App's current plan (Render spelling), or null if untiered. */
  currentPlan: string | null;
  /** The service's type — a paid-only type is never offered Free. */
  serviceType: string | null;
  /** A release has gone live; false means nothing keeps serving (w4/548). */
  live?: boolean;
}

/**
 * The Compute page's first section (w6/m483): "Under Compute, click Edit …
 * Select a new compute plan and click Save" (render.com/docs/compute-plans,
 * docs/render-artifacts/compute-page.md). It shows the current plan with an
 * Edit button; Edit opens the plan cards in place, and Cancel or a saved
 * change closes them again. The legacy-Free notice for a paid-only service
 * stays above both states, since it is about the current plan.
 */
export function ComputePlanSection({
  serviceId,
  currentPlan,
  serviceType,
  live = true,
}: ComputePlanSectionProps) {
  const [editing, setEditing] = useState(false);
  const noticeId = useId();
  const hasNotice = legacyFreeNotice(serviceType, currentPlan) !== null;

  return (
    <ComputePlanSectionFrame
      serviceType={serviceType}
      currentPlan={currentPlan}
      noticeId={noticeId}
    >
      {editing ? (
        <InstanceTypePicker
          serviceId={serviceId}
          currentPlan={currentPlan}
          serviceType={serviceType}
          live={live}
          onDone={() => setEditing(false)}
          describedBy={hasNotice ? noticeId : undefined}
        />
      ) : (
        <CurrentComputePlan
          plan={currentPlan}
          onEdit={() => setEditing(true)}
        />
      )}
    </ComputePlanSectionFrame>
  );
}

/** The collapsed section: the plan row and its Edit button, Render's layout. */
function CurrentComputePlan({
  plan,
  onEdit,
}: {
  plan: string | null;
  onEdit: () => void;
}) {
  const { t } = useTranslations();
  const { denied, reason } = useCapabilityGate("canCreate");
  return (
    <ComputePlanRowFrame
      summary={<ComputePlanSummary plan={plan} />}
      action={
        <PermissionTooltip reason={reason}>
          <Button
            variant="outline"
            size="sm"
            disabled={denied}
            onClick={onEdit}
          >
            <Pencil />
            {t("services.computePlanEdit")}
          </Button>
        </PermissionTooltip>
      }
    />
  );
}

import { Link } from "@tanstack/react-router";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ComputePlanName } from "@/common/components/compute-plan-name";
import { useTranslations } from "@/common/hooks/use-translations";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

/**
 * A service's current compute plan in one row: the plan ID beside the legacy
 * name, a separator, then its CPU/RAM (w6/m483). An untiered (bare-CR,
 * plan-less) App shows an honest "not set" state — never a fake selection —
 * since Edit still lets the operator assign one. Shared by the Compute
 * section and the Settings row.
 */
export function ComputePlanSummary({ plan }: { plan: string | null }) {
  const { t } = useTranslations();
  const { byID, loading } = useInstanceTypes();
  const current = byID(plan);

  if (loading && !current) return <Skeleton className="h-5 w-40" />;
  if (plan && !current) {
    // A plan the catalog no longer recognizes — show the raw id rather than
    // silently rendering nothing.
    return <div className="text-sm font-medium">{plan}</div>;
  }
  if (!current) {
    return (
      <div className="text-sm text-muted-foreground italic">
        {t("services.settingsNoInstanceType")}
      </div>
    );
  }
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
      <ComputePlanName plan={current} className="font-medium" />
      <span className="text-muted-foreground">|</span>
      <span className="text-muted-foreground">
        {formatInstanceCPU(current.cpu)}
      </span>
      <span className="text-muted-foreground">
        {formatInstanceMemory(current.memory)}
      </span>
    </div>
  );
}

export interface InstanceTypeRowProps {
  serviceId: string;
  /** Render's plan spelling (e.g. "pro_plus"), or null for an untiered App. */
  plan: string | null;
}

/**
 * The Settings page's "Compute Plan" row: the current plan, with an "Update"
 * link to the Compute page, where Render changes a plan (w6/m483).
 */
export function InstanceTypeRow({ serviceId, plan }: InstanceTypeRowProps) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <div className="text-sm text-muted-foreground">
          {t("services.settingsInstanceType")}
        </div>
        <div className="mt-1">
          <ComputePlanSummary plan={plan} />
        </div>
      </div>
      <Link
        to="/services/$serviceId/compute"
        params={{ serviceId }}
        className="self-start text-sm font-medium text-primary hover:underline sm:self-auto"
      >
        {t("services.settingsUpdate")}
      </Link>
    </div>
  );
}

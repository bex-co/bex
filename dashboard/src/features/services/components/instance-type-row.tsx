import { Link } from "@tanstack/react-router";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

export interface InstanceTypeRowProps {
  serviceId: string;
  /** Render's plan spelling (e.g. "pro_plus"), or null for an untiered App. */
  plan: string | null;
}

/**
 * Render's Settings-page "Instance Type" row (captured live): tier name, a
 * separator, then its CPU/RAM, with an "Update" link to the picker. An
 * untiered (bare-CR, plan-less) App shows an honest "not set" state — never a
 * fake selection — since the picker still lets the operator assign one.
 */
export function InstanceTypeRow({ serviceId, plan }: InstanceTypeRowProps) {
  const { t } = useTranslations();
  const { byID, loading } = useInstanceTypes();
  const current = byID(plan);

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <div className="text-sm text-muted-foreground">
          {t("services.settingsInstanceType")}
        </div>
        {loading && !current ? (
          <Skeleton className="mt-1 h-5 w-40" />
        ) : plan && !current ? (
          // A plan the catalog no longer recognizes — show the raw id rather
          // than silently rendering nothing.
          <div className="mt-1 text-sm font-medium">{plan}</div>
        ) : current ? (
          <div className="mt-1 flex items-center gap-2 text-sm">
            <span className="font-medium">{current.name}</span>
            <span className="text-muted-foreground">|</span>
            <span className="text-muted-foreground">
              {formatInstanceCPU(current.cpu)}
            </span>
            <span className="text-muted-foreground">
              {formatInstanceMemory(current.memory)}
            </span>
          </div>
        ) : (
          <div className="mt-1 text-sm text-muted-foreground italic">
            {t("services.settingsNoInstanceType")}
          </div>
        )}
      </div>
      <Link
        to="/services/$serviceId/plan"
        params={{ serviceId }}
        className="self-start text-sm font-medium text-primary hover:underline sm:self-auto"
      >
        {t("services.settingsUpdate")}
      </Link>
    </div>
  );
}

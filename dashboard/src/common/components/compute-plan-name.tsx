import { cn } from "@/common/lib/utils/utils.ts";
import { distinctPlanId } from "@/features/services/lib/instance-type";

/**
 * A compute plan's name as Render's dashboard shows it since its 2026-08-26
 * plan rename (docs/render-artifacts/compute-page.md, w6/m483): the plan ID
 * first ("0.5c-512mb"), the legacy name beside it ("Starter"). Without a
 * distinct plan ID it shows the name alone.
 */
export function ComputePlanName({
  plan,
  className,
}: {
  plan: { id: string; planId?: string; name: string };
  className?: string;
}) {
  const planId = distinctPlanId(plan);
  if (!planId) return <span className={className}>{plan.name}</span>;
  return (
    <span
      className={cn(
        "inline-flex flex-wrap items-baseline gap-x-2 gap-y-0.5",
        className,
      )}
    >
      <span className="font-mono">{planId}</span>{" "}
      <span className="text-sm font-normal text-muted-foreground">
        {plan.name}
      </span>
    </span>
  );
}

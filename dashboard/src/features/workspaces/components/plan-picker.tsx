import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  WORKSPACE_PLAN_CATALOG,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

export interface PlanPickerProps {
  selected: WorkspacePlanId;
  onSelect: (plan: WorkspacePlanId) => void;
}

/**
 * Render's `/new/workspace` plan cards (captured live, w6/RESEARCH-workspaces.md
 * finding 1): the post-2026-04-23 flat-rate lineup — Hobby free and capped,
 * Pro/Scale flat monthly, Enterprise custom. No payment step (Hobby needs no
 * card; bex has no billing system yet — .pm/w6/README.md "Not in w6").
 */
export function PlanPicker({ selected, onSelect }: PlanPickerProps) {
  const { t } = useTranslations();

  return (
    <div
      role="radiogroup"
      aria-label={t("workspaces.planPickerLabel")}
      className="grid grid-cols-1 gap-3 sm:grid-cols-2"
    >
      {WORKSPACE_PLAN_CATALOG.map((plan) => (
        <button
          key={plan.id}
          type="button"
          role="radio"
          aria-checked={selected === plan.id}
          onClick={() => onSelect(plan.id)}
          className={cn(
            "rounded-lg border p-4 text-left transition-colors",
            "hover:border-foreground/50",
            selected === plan.id
              ? "border-primary ring-1 ring-primary"
              : "border-border",
          )}
        >
          <div className="mb-1 flex items-baseline justify-between gap-2">
            <span className="font-semibold">{t(plan.nameKey)}</span>
            <span className="text-muted-foreground text-sm">
              {t(plan.priceKey)}
            </span>
          </div>
          <p className="text-muted-foreground text-sm">
            {t(plan.descriptionKey)}
          </p>
        </button>
      ))}
    </div>
  );
}

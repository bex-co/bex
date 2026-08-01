import { useId } from "react";
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
 * bex's workspace capability cards mirror Render's plan ids and limits without
 * copying Render's flat subscription fees. Billing starts when a resource uses
 * a non-free tier (ADR046), so the picker names the absence of a workspace fee
 * and keeps the separate resource-usage basis visible beside every plan.
 */
export function PlanPicker({ selected, onSelect }: PlanPickerProps) {
  const { t } = useTranslations();
  const usageNoteId = useId();

  return (
    <div className="space-y-2">
      <div
        role="radiogroup"
        aria-label={t("workspaces.planPickerLabel")}
        aria-describedby={usageNoteId}
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
                {t(plan.billingKey)}
              </span>
            </div>
            <p className="text-muted-foreground text-sm">
              {t(plan.descriptionKey)}
            </p>
          </button>
        ))}
      </div>
      <p id={usageNoteId} className="text-muted-foreground text-xs">
        {t("workspaces.planUsageBillingNote")}
      </p>
    </div>
  );
}

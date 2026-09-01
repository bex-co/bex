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
  disabled?: boolean;
}

/**
 * bex's workspace capability cards mirror Render's plan ids and limits, with
 * monthly fees 30% below Render's (ADR030 / pricing.yaml): Hobby $0, Pro
 * $17.50, Scale $349.30, Enterprise custom. Resource-tier usage is billed
 * separately (ADR046) and stays in the footnote under the cards.
 */
export function PlanPicker({
  selected,
  onSelect,
  disabled = false,
}: PlanPickerProps) {
  const { t } = useTranslations();
  const usageNoteId = useId();

  return (
    <div className="space-y-3">
      <div
        role="radiogroup"
        aria-label={t("workspaces.planPickerLabel")}
        aria-describedby={usageNoteId}
        className="grid grid-cols-1 gap-4 sm:grid-cols-2"
      >
        {WORKSPACE_PLAN_CATALOG.map((plan) => {
          const checked = selected === plan.id;
          return (
            <button
              key={plan.id}
              type="button"
              role="radio"
              aria-checked={checked}
              disabled={disabled}
              onClick={() => onSelect(plan.id)}
              className={cn(
                "flex min-h-56 flex-col rounded-xl border p-5 text-left transition-colors",
                "hover:border-foreground/50 disabled:cursor-not-allowed disabled:opacity-60",
                checked
                  ? "border-primary ring-2 ring-primary"
                  : "border-border",
              )}
            >
              <div className="mb-2 flex items-baseline justify-between gap-2">
                <span className="text-lg font-semibold">{t(plan.nameKey)}</span>
                <span className="text-muted-foreground text-sm font-medium">
                  {t(plan.billingKey)}
                </span>
              </div>
              <p className="text-muted-foreground mb-3 text-sm">
                {t(plan.descriptionKey)}
              </p>
              <ul className="text-muted-foreground mb-4 list-disc space-y-1 pl-4 text-sm">
                {plan.bulletKeys.map((key) => (
                  <li key={key}>{t(key)}</li>
                ))}
              </ul>
              <span
                className={cn(
                  "mt-auto text-sm font-medium",
                  checked ? "text-primary" : "text-muted-foreground",
                )}
              >
                {checked
                  ? t("workspaces.planSelected")
                  : t("workspaces.planSelect")}
              </span>
            </button>
          );
        })}
      </div>
      <p id={usageNoteId} className="text-muted-foreground text-xs">
        {t("workspaces.planUsageBillingNote")}
      </p>
    </div>
  );
}

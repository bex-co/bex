import { cn } from "@/common/lib/utils/utils.ts";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

/** The subset of a catalog tier the cards render — services and Key Value both satisfy it. */
export interface PlanCardTier {
  id: string;
  name: string;
  cpu: string;
  memory: string;
}

export interface PlanCardGridProps {
  instanceTypes: PlanCardTier[];
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
  ariaLabel?: string;
}

/**
 * The create forms' plan-tier cards, matching Render's captured `/new/redis`
 * instance-type radio cards (docs/render-artifacts/key-value.md) — one card per
 * catalog plan, RAM + connection-style specs, current selection highlighted.
 * A plain button grid rather than a Radix RadioGroup: the codebase has no
 * radio-group primitive yet and one card-button set doesn't warrant adding the
 * dependency; `role="radio"` + `aria-checked` keep it accessible.
 */
export function PlanCardGrid({
  instanceTypes,
  value,
  onChange,
  disabled = false,
  ariaLabel,
}: PlanCardGridProps) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      aria-disabled={disabled || undefined}
      className="grid grid-cols-1 gap-3 sm:grid-cols-3"
    >
      {instanceTypes.map((it) => {
        const selected = it.id === value;
        return (
          <button
            key={it.id}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={disabled}
            onClick={() => onChange(it.id)}
            className={cn(
              "rounded-lg border p-3 text-left transition-colors",
              selected
                ? "border-primary ring-1 ring-primary"
                : "border-border hover:border-muted-foreground/50",
            )}
          >
            <div className="font-medium">{it.name}</div>
            <div className="text-sm text-muted-foreground">
              {formatInstanceMemory(it.memory)} RAM ·{" "}
              {formatInstanceCPU(it.cpu)}
            </div>
          </button>
        );
      })}
    </div>
  );
}

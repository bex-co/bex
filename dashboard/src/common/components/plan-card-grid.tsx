import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

export const CREATE_PLAN_CARD_GRID_CLASS =
  "grid grid-cols-1 gap-3 sm:grid-cols-3";

/** The subset of a catalog tier the cards render — services and Key Value both satisfy it. */
export interface PlanCardTier {
  id: string;
  name: string;
  cpu: string;
  memory: string;
  monthlyUsd?: string;
  /** The rate per minute of run time, shown when the grid prices run time. */
  usdPerMinute?: string;
  /**
   * False for a temporarily paused tier (w2/m250): the card stays in place,
   * disabled, so the grid's geometry never changes; omitted means available.
   */
  available?: boolean;
  /** bex-api's reason a tier is unavailable — the disabled card's tooltip. */
  unavailableReason?: string;
}

export interface PlanCardGridProps {
  instanceTypes: PlanCardTier[];
  value: string;
  onChange: (id: string) => void;
  disabled?: boolean;
  /** The group's accessible name — the visible plan label it sits under. */
  ariaLabel: string;
  /**
   * Price each tier per minute rather than per month: a cron job is billed
   * per second only while a run executes (w4/546). The caller says so.
   */
  runTimePricing?: boolean;
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
  runTimePricing = false,
}: PlanCardGridProps) {
  const { t } = useTranslations();
  const radio = useCardRadioGroup(
    instanceTypes.filter((it) => it.available !== false).map((it) => it.id),
    value,
    onChange,
    disabled,
  );
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      aria-disabled={disabled || undefined}
      className={CREATE_PLAN_CARD_GRID_CLASS}
    >
      {instanceTypes.map((it) => {
        const selected = it.id === value;
        const unavailable = it.available === false;
        // The short per-minute form: the price shares the card's top row
        // with the name, and the form's hint says it is run time (w4/546).
        const price = runTimePricing
          ? it.usdPerMinute &&
            t("common.pricePerMinute", { price: it.usdPerMinute })
          : it.monthlyUsd &&
            t("common.pricePerMonth", { price: it.monthlyUsd });
        return (
          <button
            key={it.id}
            {...radio(it.id)}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={disabled || unavailable}
            title={unavailable ? it.unavailableReason : undefined}
            onClick={() => onChange(it.id)}
            className={cn(
              "rounded-lg border p-3 text-left transition-colors",
              selected
                ? "border-primary ring-1 ring-primary"
                : "border-border hover:border-muted-foreground/50",
              unavailable && "cursor-not-allowed opacity-60",
            )}
          >
            <div className="flex items-baseline justify-between gap-2">
              <div className="font-medium">{it.name}</div>
              {unavailable ? (
                // In the price's slot, so a paused card keeps its height.
                <div className="shrink-0 text-sm text-muted-foreground">
                  {t("common.planUnavailable")}
                  {it.unavailableReason ? (
                    <span className="sr-only">{it.unavailableReason}</span>
                  ) : null}
                </div>
              ) : price ? (
                <div className="shrink-0 text-sm font-semibold tabular-nums">
                  {price}
                </div>
              ) : null}
            </div>
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

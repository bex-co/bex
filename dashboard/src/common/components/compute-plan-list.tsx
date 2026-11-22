import { useMemo, useState } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { ComputePlanName } from "@/common/components/compute-plan-name";
import { RadioDot } from "@/common/components/radio-dot";
import { Button } from "@/common/components/ui/button";
import {
  computePlanRowClasses,
  planListCollapses,
  planPriceLabel,
} from "@/common/lib/plan-list";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

/** The subset of a catalog tier a row renders — services, Postgres and Key
 *  Value all satisfy it. */
export interface ComputePlanTier {
  id: string;
  /** Render's compute plan ID, shown beside the name (w6/m483); services only. */
  planId?: string;
  name: string;
  cpu: string;
  memory: string;
  monthlyUsd?: string;
  /** The rate per minute of run time, shown when the list prices run time. */
  usdPerMinute?: string;
  /**
   * The plan's client connection limit, a column of its own after the RAM as
   * on Render's Key Value plans (w6/m459); omitted for a family that shows
   * none (services, Postgres).
   */
  connectionLimit?: number;
  /**
   * False for a temporarily paused tier (w2/m250): the row stays in place,
   * disabled, so the list's geometry never changes; omitted means available.
   */
  available?: boolean;
  /** bex-api's reason a tier is unavailable — the disabled row's tooltip. */
  unavailableReason?: string;
}

export interface ComputePlanListProps {
  instanceTypes: ComputePlanTier[];
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
  /**
   * Show this many tiers (and the selected one) until "Show all plans" is
   * pressed — Render's create forms show four. Omitted, every tier shows.
   */
  collapsedCount?: number;
}

/** How many plans a create page's list shows before "Show all plans", as on
 *  Render's create forms. */
export const CREATE_PLAN_ROWS = 4;

/** The list frame and one row's box, shared with the loading slots
 *  (`PlanTierSlots`) so a list keeps its geometry while the catalog loads. */
export const COMPUTE_PLAN_LIST_CLASS =
  "@container rounded-lg border p-1.5 space-y-1";
export const COMPUTE_PLAN_ROW_CLASS =
  "flex min-h-12 w-full items-center gap-3 rounded-md border px-3 py-2 text-left";

/**
 * Render's compute-plan radio list (render.com/web/new Compute, /new/redis,
 * /new/database, live captures 2026-11-21): one row per catalog plan with its
 * name, CPU, RAM, connection limit where the tier carries one, and price; the
 * selected row outlined. Rows stack their specs under the name on a narrow
 * list and line them up as columns on a wide one — the list measures its own
 * width, so it never squeezes a name into one letter per line.
 * Plain buttons rather than a Radix RadioGroup; `role="radio"` +
 * `aria-checked` and the W3C arrow-key contract (useCardRadioGroup) keep it
 * accessible.
 */
export function ComputePlanList({
  instanceTypes,
  value,
  onChange,
  disabled = false,
  ariaLabel,
  runTimePricing = false,
  collapsedCount,
}: ComputePlanListProps) {
  const { t, i18n } = useTranslations();
  const [expanded, setExpanded] = useState(false);
  const number = useMemo(
    () => new Intl.NumberFormat(i18n.language),
    [i18n.language],
  );
  const collapses = planListCollapses(instanceTypes.length, collapsedCount);
  const shown =
    collapses && !expanded
      ? instanceTypes.filter(
          (it, index) => index < (collapsedCount ?? 0) || it.id === value,
        )
      : instanceTypes;
  const row = computePlanRowClasses(
    instanceTypes.some((it) => it.connectionLimit),
  );
  const radio = useCardRadioGroup(
    shown.filter((it) => it.available !== false).map((it) => it.id),
    value,
    onChange,
    disabled,
  );
  return (
    <div className={COMPUTE_PLAN_LIST_CLASS}>
      <div
        role="radiogroup"
        aria-label={ariaLabel}
        aria-disabled={disabled || undefined}
        className="space-y-1"
      >
        {shown.map((it) => {
          const selected = it.id === value;
          const unavailable = it.available === false;
          const price = planPriceLabel(it, runTimePricing, t);
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
                COMPUTE_PLAN_ROW_CLASS,
                // Focus in the neutral foreground, offset: it must read apart
                // from the green selected border.
                "flex-wrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground",
                selected
                  ? "border-primary bg-primary/5"
                  : "border-transparent hover:bg-muted/60",
                unavailable && "cursor-not-allowed",
              )}
            >
              <RadioDot
                checked={selected}
                className={unavailable ? "opacity-60" : undefined}
              />
              <span className={cn(row.specs, unavailable && "opacity-60")}>
                <ComputePlanName
                  plan={it}
                  className={cn(row.name, "[&_.font-mono]:text-[0.875em]")}
                />
                <span className={row.specLine}>
                  <span className={row.cpu}>{formatInstanceCPU(it.cpu)}</span>
                  <span className={row.ram}>
                    {formatInstanceMemory(it.memory)} RAM
                  </span>
                  {it.connectionLimit ? (
                    <span className={row.conn}>
                      {t("common.planConnections", {
                        count: it.connectionLimit,
                        formatted: number.format(it.connectionLimit),
                      })}
                    </span>
                  ) : null}
                </span>
              </span>
              {unavailable ? (
                // In the price's slot, so a paused row keeps its shape.
                <span
                  className={cn(row.price, "text-muted-foreground font-normal")}
                >
                  {t("common.planUnavailable")}
                </span>
              ) : (
                <span className={row.price}>{price}</span>
              )}
              {unavailable && it.unavailableReason ? (
                // Why, on a line of its own under the row's columns, at the
                // muted text's full contrast (the row itself is dimmed).
                <span className="text-muted-foreground basis-full ps-7 text-sm">
                  {it.unavailableReason}
                </span>
              ) : null}
            </button>
          );
        })}
      </div>
      {collapses ? (
        <div className="px-1.5 pt-1 pb-0.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-11 sm:h-9"
            aria-expanded={expanded}
            onClick={() => setExpanded(!expanded)}
          >
            <ChevronDown
              className={cn(
                "transition-transform motion-reduce:transition-none",
                expanded && "rotate-180",
              )}
            />
            {t(
              expanded ? "common.planListShowFewer" : "common.planListShowAll",
            )}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

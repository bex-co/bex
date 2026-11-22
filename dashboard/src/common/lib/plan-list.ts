import type { Translate } from "@/common/hooks/use-translations";

/** Whether a plan list of `count` tiers folds any away at `collapsedCount`
 *  (ComputePlanList, PlanTierSlots): a fold that would hide a single row is
 *  not worth its "Show all plans" button. */
export function planListCollapses(
  count: number,
  collapsedCount: number | undefined,
): boolean {
  return collapsedCount !== undefined && count > collapsedCount + 1;
}

/**
 * A row's layout classes: the specs stacked under the name on a narrow list,
 * columns on a wide one. A list with a connection-limit column (Key Value)
 * has one more column, so it lines them up only from `@3xl` (768 px) — below
 * that "Standard" broke mid-word at 1280 px — and stacks before.
 */
export function computePlanRowClasses(connections: boolean) {
  return connections
    ? {
        specs: "min-w-0 flex-1 @3xl:flex @3xl:items-center @3xl:gap-4",
        name: "font-medium whitespace-nowrap @3xl:min-w-0 @3xl:flex-1",
        specLine:
          "text-muted-foreground flex flex-wrap gap-x-3 text-sm @3xl:contents",
        cpu: "@3xl:w-20 @3xl:shrink-0",
        ram: "@3xl:w-24 @3xl:shrink-0",
        conn: "@3xl:w-36 @3xl:shrink-0",
        price:
          "shrink-0 text-right text-sm font-semibold tabular-nums @3xl:w-28",
      }
    : {
        specs: "min-w-0 flex-1 @lg:flex @lg:items-center @lg:gap-4",
        name: "font-medium whitespace-nowrap @lg:min-w-0 @lg:flex-1",
        specLine:
          "text-muted-foreground flex flex-wrap gap-x-3 text-sm @lg:contents",
        cpu: "@lg:w-20 @lg:shrink-0",
        ram: "@lg:w-24 @lg:shrink-0",
        conn: "@lg:w-36 @lg:shrink-0",
        price:
          "shrink-0 text-right text-sm font-semibold tabular-nums @lg:w-28",
      };
}

/** A tier's price as the plan list and the action bar print it: per minute
 *  of run time when the list prices run time (a cron job), else per month. */
export function planPriceLabel(
  plan: { monthlyUsd?: string; usdPerMinute?: string },
  runTimePricing: boolean,
  t: Translate,
): string | undefined {
  if (runTimePricing) {
    return plan.usdPerMinute
      ? t("common.pricePerMinute", { price: plan.usdPerMinute })
      : undefined;
  }
  return plan.monthlyUsd
    ? t("common.pricePerMonth", { price: plan.monthlyUsd })
    : undefined;
}

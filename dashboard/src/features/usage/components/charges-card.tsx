// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useMemo, useState } from "react";
import { AlertTriangle, ChevronRight } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { periodLabel } from "@/features/usage/lib/period";
import {
  billingWindow,
  currentPeriod,
  groupByCategory,
  money,
  projectMonthEnd,
  projectPeriodEnd,
  usd,
  type ChargeCategory,
} from "@/features/usage/lib/charges";
import type {
  ChargeLine,
  Coverage,
  EstimatedCost,
  ResourceEstimate,
} from "../hooks/use-usage";

/**
 * The charge tree: category → resource → charge line. This is the page's one
 * consumption view. It replaced four sibling tables (compute / bandwidth /
 * build / storage) that listed the same resources four times over and never
 * said what any of it cost — here money is the spine and usage is a column, so
 * "what am I paying for" and "how much did it consume" are one question.
 */

const CATEGORY_LABEL_KEYS: Record<string, string> = {
  service: "usage.categoryServices",
  postgres: "usage.categoryPostgres",
  key_value: "usage.categoryKeyValue",
  sandbox: "usage.categorySandboxes",
};

/** Meter kinds read as charge names on a bill, not as raw meter identifiers. */
const CHARGE_LABEL_KEYS: Record<string, string> = {
  instance_seconds: "usage.chargeCompute",
  egress_bytes: "usage.chargeBandwidth",
  build_seconds: "usage.chargeBuild",
  storage_gb_seconds: "usage.chargeStorage",
  sandbox_compute_seconds: "usage.chargeSandboxCompute",
};

function ChargeRow({ charge }: { charge: ChargeLine }) {
  const { t } = useTranslations();
  const labelKey = CHARGE_LABEL_KEYS[charge.kind];
  return (
    <div className="grid gap-x-4 gap-y-1 py-2 sm:grid-cols-[minmax(0,1fr)_auto_auto_auto] sm:items-baseline">
      <span className="text-sm">
        {labelKey ? t(labelKey) : charge.kind}
        {charge.tier && (
          <span className="ml-2 text-xs text-muted-foreground capitalize">
            {charge.tier}
          </span>
        )}
      </span>
      <span className="font-mono text-xs text-muted-foreground tabular-nums sm:w-32 sm:text-right">
        {charge.rateUsd === "0"
          ? t("usage.chargeFree")
          : `$${charge.rateUsd}/${charge.unit}`}
      </span>
      <span className="font-mono text-xs text-muted-foreground tabular-nums sm:w-28 sm:text-right">
        {charge.quantity} {charge.unit}
      </span>
      <span className="font-mono text-sm tabular-nums sm:w-20 sm:text-right">
        {money(usd(charge.costUsd))}
      </span>
    </div>
  );
}

function ResourceRow({ resource }: { resource: ResourceEstimate }) {
  const [open, setOpen] = useState(false);
  const label = resource.serviceName || resource.serviceId;
  return (
    <div className="border-t first:border-t-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-muted/50"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3.5 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
          )}
        />
        <span className="min-w-0 flex-1 truncate text-sm font-medium">
          {label}
        </span>
        <span className="font-mono text-sm tabular-nums">
          {money(usd(resource.costUsd))}
        </span>
      </button>
      {open && (
        <div className="divide-y border-t bg-muted/20 px-3 py-1 pl-9">
          {resource.charges.map((c) => (
            <ChargeRow key={`${c.kind}:${c.tier}`} charge={c} />
          ))}
        </div>
      )}
    </div>
  );
}

function CategoryRow({
  category,
  expandAll,
}: {
  category: ChargeCategory;
  expandAll: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const labelKey = CATEGORY_LABEL_KEYS[category.key];
  const expanded = open || expandAll;
  return (
    <div className="rounded-md border">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={expanded}
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left hover:bg-muted/50"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-4 shrink-0 text-muted-foreground transition-transform",
            expanded && "rotate-90",
          )}
        />
        <span className="min-w-0 flex-1 truncate font-medium">
          {labelKey ? t(labelKey) : category.key}
        </span>
        <span className="font-mono tabular-nums">
          {money(category.totalUsd)}
        </span>
      </button>
      {expanded && (
        <div className="border-t">
          {category.resources.map((r) => (
            <ResourceRow
              key={`${r.resourceKind}:${r.serviceId}`}
              resource={r}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * The honesty caveat for a money figure built on incomplete metering (w4/048).
 * Mirrors the Metrics page's degraded-source badge (network-metrics-card) so the
 * two surfaces disclose degraded data the same way: an amber "Partial data" line
 * whose tooltip names the coverage boundary and the degraded meters. Rendered
 * only when the estimate is genuinely not complete and there is something
 * concrete to disclose — a `through` bound or named degraded sources — so an
 * indeterminate "unknown" read with nothing to show stays silent.
 */
function CoverageCaveat({ coverage }: { coverage: Coverage }) {
  const { t } = useTranslations();
  const parts = [t("usage.coveragePartialLead")];
  if (coverage.through) {
    // Date only, sliced from the UTC RFC3339 string: deterministic across the
    // SSR/hydration boundary, unlike a locale/timezone-formatted date.
    parts.push(
      t("usage.coveragePartialThrough", { through: coverage.through.slice(0, 10) }),
    );
  }
  if (coverage.degradedSources.length > 0) {
    parts.push(
      t("usage.coveragePartialSources", {
        sources: coverage.degradedSources.join(", "),
      }),
    );
  }
  return (
    <span
      className="mt-1 flex items-center gap-1 text-xs text-amber-600 dark:text-amber-500"
      title={parts.join(" ")}
    >
      <AlertTriangle className="h-3.5 w-3.5" />
      {t("usage.coveragePartial")}
    </span>
  );
}

export interface ChargesCardProps {
  estimatedCost: EstimatedCost | null;
  /**
   * Metering completeness for the period (ADR040). When the estimate is not
   * complete and there is something to disclose, the card caveats the totals so
   * a degraded/partial estimate isn't presented as authoritative (w4/048).
   */
  coverage?: Coverage | null;
  /** Stripe's gross rated charge for the period; the total once Stripe has rated anything. */
  invoicedUsd: string | null;
  /** What Stripe actually collects after credits and comp discounts; shown when it differs. */
  amountDueUsd?: string | null;
  /**
   * RFC3339 bounds of the subscription period `invoicedUsd` accrued over.
   * Stripe rates the subscription period (e.g. the 16th → the 16th), not the
   * calendar month, so a rated total must be projected over this window.
   */
  ratedPeriodStart?: string | null;
  ratedPeriodEnd?: string | null;
  loading: boolean;
  /** The period on screen, "YYYY-MM". Projection only applies to the current one. */
  period: string;
  /** Injected for tests; defaults to now. */
  now?: Date;
}

/**
 * Unbilled charges for the period. The header total prefers Stripe's real
 * current-period amount when the workspace has a subscription and falls back to
 * the price-sheet estimate otherwise — one number in one place, labelled for
 * which it is, rather than the two competing "estimated cost" and "current
 * spend" cards this replaced.
 */
export function ChargesCard({
  estimatedCost,
  coverage = null,
  invoicedUsd,
  amountDueUsd = null,
  ratedPeriodStart = null,
  ratedPeriodEnd = null,
  loading,
  period,
  now = new Date(),
}: ChargesCardProps) {
  const { t } = useTranslations();
  const hydrated = useIsHydrated();
  const [expandAll, setExpandAll] = useState(false);

  const categories = useMemo(
    () => groupByCategory(estimatedCost?.resources ?? []),
    [estimatedCost],
  );

  const invoiced = invoicedUsd == null ? null : usd(invoicedUsd);
  // Zero is not a rating. Stripe prices the period from meter events that land
  // asynchronously, so a workspace can hold a real charge tree while Stripe's
  // gross charge is still 0 — and "$0.00 month to date" above a tree summing to
  // $74 is the contradiction this card exists to avoid (w6/m98). The estimate
  // carries the total until Stripe has a figure of its own.
  const rated = invoiced != null && invoiced > 0 ? invoiced : null;
  // The estimate total is summed from the categories on screen rather than
  // taken from `estimatedCost.totalUsd`. The two differ by a cent or so — the
  // backend rounds the raw total once, the tree rounds every resource — and a
  // page whose parts visibly fail to add up to its own total reads as a bug
  // even when both numbers are defensible. A rated amount is Stripe's own
  // rating, not ours, so it is shown verbatim; the tree explains it rather
  // than deriving it.
  const total = rated ?? categories.reduce((sum, c) => sum + c.totalUsd, 0);
  // Credit grants and Mode B comps sit between the charge and the bill. The
  // charge stays the headline — it is what the tree adds up to — and the
  // amount actually collected gets its own line, but only when the two differ.
  const due = amountDueUsd == null ? null : usd(amountDueUsd);
  const dueAfterCredit =
    rated != null && due != null && due !== rated ? due : null;

  const isCurrentMonth = period === "" || period === currentPeriod(now);
  // A rated total accrued over Stripe's subscription period, not the calendar
  // month; projecting it over the month's elapsed fraction understated the
  // figure ~2.4× on a mid-month-anchored subscription (w6/050). Each total is
  // projected over the window it actually covers — the subscription period for
  // the rated figure (and only when its bounds are known), the calendar month
  // for the category-sum fallback.
  //
  // Gated behind `hydrated` because `now` defaults to a live clock read,
  // evaluated independently at SSR and at hydration — a continuously-changing
  // elapsed ratio rendered as text can never agree between the two passes
  // (React #418, w6/049). SSR emits no projection row; it appears once the
  // client render owns the clock, the same deferral `LocalDateTime` uses
  // (w6/030).
  const ratedWindow = billingWindow(ratedPeriodStart, ratedPeriodEnd);
  const projected =
    !hydrated || !isCurrentMonth
      ? null
      : rated != null
        ? ratedWindow &&
          projectPeriodEnd(total, now, ratedWindow.start, ratedWindow.end)
        : projectMonthEnd(total, now);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.chargesTitle")}</CardTitle>
        <CardDescription>
          {/* Say nothing rather than the wrong thing. `invoiced == null` means
              two different things — "this workspace has no Stripe pricing" and
              "the number has not arrived yet" — and the card used to read the
              second as the first, asserting "An estimate, not an invoice."
              before swapping to "The total is the amount Stripe will invoice."
              a moment later. On a page about real money, committing to a claim
              and then contradicting it is worse than a beat of silence, so
              while the figure is still resolving the description is neutral
              and settles on exactly one answer (w10/m11/t001). */}
          {loading && invoicedUsd == null
            ? t("usage.chargesDescriptionPending")
            : rated == null
              ? t("usage.chargesDescriptionEstimate")
              : t("usage.chargesDescriptionInvoiced")}
        </CardDescription>
        {coverage &&
        coverage.state !== "complete" &&
        (coverage.through !== "" || coverage.degradedSources.length > 0) ? (
          <CoverageCaveat coverage={coverage} />
        ) : null}
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && categories.length === 0 ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-11 w-full rounded-md" />
            ))}
          </div>
        ) : categories.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.chargesEmpty")}
          </p>
        ) : (
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setExpandAll((v) => !v)}
            >
              {expandAll ? t("usage.collapseAll") : t("usage.expandAll")}
            </Button>
            <div className="space-y-2">
              {categories.map((c) => (
                <CategoryRow key={c.key} category={c} expandAll={expandAll} />
              ))}
            </div>
          </>
        )}

        <dl className="space-y-1 border-t pt-4">
          <div className="flex items-baseline justify-between gap-4">
            {/* `isCurrentMonth` reads the same live clock as the projection but
                only flips at a month boundary — and only for an explicit
                period, which the page never hydrates with (it starts on
                period="", clock-independent). A mount-gate would flash the
                wrong label on every load to guard that unreachable sub-second
                race, so the divergence is suppressed instead; the trade is
                that a straddling render would keep the server's label
                (w6/049). */}
            <dt className="font-medium" suppressHydrationWarning>
              {isCurrentMonth
                ? t("usage.totalToDate")
                : t("usage.totalForPeriod")}
            </dt>
            <dd className="font-mono text-lg font-semibold tabular-nums">
              {money(total)} USD
            </dd>
          </div>
          {dueAfterCredit != null && (
            <div className="flex items-baseline justify-between gap-4 text-sm text-muted-foreground">
              <dt>{t("usage.amountDueAfterCredits")}</dt>
              <dd className="font-mono tabular-nums">
                {money(dueAfterCredit)} USD
              </dd>
            </div>
          )}
          {projected != null && (
            <div className="flex items-baseline justify-between gap-4 text-sm text-muted-foreground">
              {/* A subscription period need not align with the calendar month
                  (it can span two), so a rated projection names the billing
                  period rather than a month it may not cover (w6/050). */}
              <dt>
                {rated != null
                  ? t("usage.projectedTotalBillingPeriod")
                  : t("usage.projectedTotal", {
                      month:
                        periodLabel(currentPeriod(now)).split(" ")[0] ?? "",
                    })}
              </dt>
              <dd className="font-mono tabular-nums">{money(projected)} USD</dd>
            </div>
          )}
        </dl>
      </CardContent>
    </Card>
  );
}

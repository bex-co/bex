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
import { ChevronRight } from "lucide-react";
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
import { periodLabel } from "@/features/usage/lib/period";
import {
  currentPeriod,
  groupByCategory,
  money,
  projectMonthEnd,
  usd,
  type ChargeCategory,
} from "@/features/usage/lib/charges";
import type {
  ChargeLine,
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

export interface ChargesCardProps {
  estimatedCost: EstimatedCost | null;
  /** Stripe's real current-period amount; shown as the total when present. */
  invoicedUsd: string | null;
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
  invoicedUsd,
  loading,
  period,
  now = new Date(),
}: ChargesCardProps) {
  const { t } = useTranslations();
  const [expandAll, setExpandAll] = useState(false);

  const categories = useMemo(
    () => groupByCategory(estimatedCost?.resources ?? []),
    [estimatedCost],
  );

  const invoiced = invoicedUsd == null ? null : usd(invoicedUsd);
  // The estimate total is summed from the categories on screen rather than
  // taken from `estimatedCost.totalUsd`. The two differ by a cent or so — the
  // backend rounds the raw total once, the tree rounds every resource — and a
  // page whose parts visibly fail to add up to its own total reads as a bug
  // even when both numbers are defensible. An invoiced amount is Stripe's
  // rating, not ours, so it is shown verbatim; the tree explains it rather
  // than deriving it.
  const total = invoiced ?? categories.reduce((sum, c) => sum + c.totalUsd, 0);

  const isCurrentMonth = period === "" || period === currentPeriod(now);
  const projected = isCurrentMonth ? projectMonthEnd(total, now) : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.chargesTitle")}</CardTitle>
        <CardDescription>
          {invoiced == null
            ? t("usage.chargesDescriptionEstimate")
            : t("usage.chargesDescriptionInvoiced")}
        </CardDescription>
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
            <dt className="font-medium">
              {isCurrentMonth
                ? t("usage.totalToDate")
                : t("usage.totalForPeriod")}
            </dt>
            <dd className="font-mono text-lg font-semibold tabular-nums">
              {money(total)} USD
            </dd>
          </div>
          {projected != null && (
            <div className="flex items-baseline justify-between gap-4 text-sm text-muted-foreground">
              <dt>
                {t("usage.projectedTotal", {
                  month: periodLabel(currentPeriod(now)).split(" ")[0] ?? "",
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

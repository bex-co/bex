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

import { useState } from "react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { periodFor, periodLabel } from "@/features/usage/lib/period";
import { useUsage } from "../hooks/use-usage";
import { WorkspaceResourceCaps } from "./resource-caps";
import { PlanCard } from "./plan-card";
import { PaymentMethodCard } from "./payment-method-card";
import { ChargesCard } from "./charges-card";
import { CreditBalanceCard, InvoiceHistoryCard } from "./billing-summary-cards";
import { UsageNavigation } from "./usage-navigation";

/**
 * The workspace billing page (`/billing`).
 *
 * Six cards, each answering one question a person opens this page with: what
 * plan am I on, what will you charge, what am I allowed to run, what is this
 * month costing me and why, what credit do I hold, and what have I paid. The
 * charge tree (ChargesCard) carries the consumption detail that four separate
 * per-meter usage tables used to spread across the page without ever pricing
 * it.
 */

/** Build `count` month options starting `startFrom` months back, newest first. */
function buildMonthOptions(
  count: number,
  startFrom = 0,
): { value: string; label: string }[] {
  return Array.from({ length: count }, (_, i) => {
    const p = periodFor(i + startFrom);
    return { value: p, label: periodLabel(p) };
  });
}

const MONTH_OPTIONS = buildMonthOptions(5, 1);
// Sentinel value for Radix Select (an empty string is not a legal item value).
const CURRENT_MONTH_SENTINEL = "__current__";

export function UsagePage() {
  const { t } = useTranslations();
  const [selectedPeriod, setSelectedPeriod] = useState<string>(
    CURRENT_MONTH_SENTINEL,
  );
  const period =
    selectedPeriod === CURRENT_MONTH_SENTINEL ? undefined : selectedPeriod;

  const { summary, loading, error } = useUsage(period);
  const billing = summary?.billing ?? null;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto grid w-full max-w-5xl items-start gap-8 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
          <div className="min-w-0 space-y-6">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h1 className="text-xl font-semibold">
                  {t("usage.pageTitle")}
                </h1>
                <p className="text-sm text-muted-foreground">
                  {t("usage.pageSubtitle")}
                </p>
              </div>
              <Select value={selectedPeriod} onValueChange={setSelectedPeriod}>
                <SelectTrigger
                  size="sm"
                  className="w-44"
                  aria-label={t("usage.monthPickerLabel")}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={CURRENT_MONTH_SENTINEL}>
                    {t("usage.currentMonth")}
                  </SelectItem>
                  {MONTH_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Mobile section nav — the desktop right rail renders below. */}
            <UsageNavigation className="sticky top-0 z-20 -mx-4 border-y bg-background/95 px-4 py-2 backdrop-blur sm:-mx-6 sm:px-6 lg:hidden" />

            {error && !summary && (
              <Alert variant="destructive">
                <AlertTitle>{t("usage.errorTitle")}</AlertTitle>
                <AlertDescription>{error.message}</AlertDescription>
              </Alert>
            )}

            <section id="plan" className="scroll-mt-6">
              <PlanCard />
            </section>
            <section id="payment-method" className="scroll-mt-6">
              <PaymentMethodCard />
            </section>
            <section id="included-usage" className="scroll-mt-6">
              <WorkspaceResourceCaps />
            </section>
            <section id="charges" className="scroll-mt-6">
              <ChargesCard
                estimatedCost={summary?.estimatedCost ?? null}
                invoicedUsd={billing?.currentCost?.amountUsd ?? null}
                amountDueUsd={billing?.currentCost?.amountDueUsd ?? null}
                loading={loading}
                period={period ?? ""}
              />
            </section>
            <section id="credit-balance" className="scroll-mt-6">
              <CreditBalanceCard billing={billing} />
            </section>
            <section id="invoice-history" className="scroll-mt-6">
              <InvoiceHistoryCard billing={billing} />
            </section>
          </div>

          {/* Desktop right rail — same quick nav as the service settings page. */}
          <UsageNavigation className="sticky top-6 hidden lg:block" />
        </div>
      </div>
    </DashboardLayout>
  );
}

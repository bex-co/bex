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

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatDateISO } from "@/common/lib/format";
import { periodLabel } from "@/features/usage/lib/period";
import type { Billing, BillingCredits } from "../hooks/use-usage";

/**
 * The two money cards that read straight off Stripe: remaining credit, and the
 * finalized invoice history.
 */

/** Display-side USD math on the backend's normalized "12.34" strings. */
function usd(amount: string): number {
  const n = Number.parseFloat(amount);
  return Number.isFinite(n) ? n : 0;
}

/**
 * Credit balance. Renders only when the workspace holds credit — the backend
 * omits the block at zero balance, and an always-present "$0.00 in credits"
 * card is noise for everyone who has never had a grant.
 *
 * Takes the whole billing block rather than just the credits, so it can show
 * what the balance actually does to this period's bill. That arithmetic used
 * to sit on a separate current-spend card; the question it answers ("do I owe
 * anything?") belongs next to the balance, not next to the charges.
 */
export function CreditBalanceCard({ billing }: { billing: Billing | null }) {
  const { t } = useTranslations();
  const credits: BillingCredits | null = billing?.credits ?? null;
  if (!credits) return null;
  const expiring = credits.grants.find((g) => g.expiresAt !== "");
  const cost = billing?.currentCost ? usd(billing.currentCost.amountUsd) : null;
  const available = usd(credits.availableUsd);
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.creditsTitle")}</CardTitle>
        <CardDescription>{t("usage.creditsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-xs tracking-wide text-muted-foreground uppercase">
          {t("usage.creditsTotalLabel")}
        </p>
        <p className="font-mono text-3xl font-semibold tabular-nums">
          ${credits.availableUsd}
        </p>
        {cost != null && (
          <p className="mt-1 text-sm text-muted-foreground tabular-nums">
            {t("usage.creditsAppliedLine", {
              applied: Math.min(available, cost).toFixed(2),
              due: Math.max(0, cost - available).toFixed(2),
            })}
          </p>
        )}
        {expiring && (
          <p className="mt-2 text-xs text-muted-foreground">
            {t("usage.creditsExpiryNote", {
              amount: expiring.remainingUsd,
              date: formatDateISO(expiring.expiresAt) ?? expiring.expiresAt,
            })}
          </p>
        )}
        {/* ADR046: credit pays the invoice first, but a card is still required. */}
        <p className="mt-2 text-xs text-muted-foreground">
          {t("usage.creditsCardStillRequired")}
        </p>
      </CardContent>
    </Card>
  );
}

/** "YYYY-MM" for an RFC3339 instant, for labelling an invoice by its month. */
function invoiceMonth(periodStart: string): string {
  if (periodStart.length < 7) return "";
  return periodLabel(periodStart.slice(0, 7));
}

/**
 * Finalized invoice history. Renders only with real invoices: a workspace on
 * the price-sheet estimate alone has none, and an empty table would imply
 * something is missing rather than not yet applicable.
 */
export function InvoiceHistoryCard({ billing }: { billing: Billing | null }) {
  const { t } = useTranslations();
  const invoices = billing?.invoices ?? [];
  if (invoices.length === 0) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.invoiceHistoryTitle")}</CardTitle>
        <CardDescription>
          {t("usage.invoiceHistoryDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b text-xs tracking-wide text-muted-foreground uppercase">
              <th scope="col" className="py-2 text-left font-medium">
                {t("usage.colInvoicePeriod")}
              </th>
              <th scope="col" className="py-2 text-left font-medium">
                {t("usage.colInvoiceStatus")}
              </th>
              <th scope="col" className="py-2 text-right font-medium">
                {t("usage.colInvoiceAmount")}
              </th>
            </tr>
          </thead>
          <tbody>
            {invoices.map((inv) => (
              <tr key={inv.id} className="border-b last:border-b-0">
                <td className="py-2.5 font-medium">
                  {invoiceMonth(inv.periodStart) || "—"}
                </td>
                <td className="py-2.5">
                  <Badge
                    variant={inv.status === "paid" ? "secondary" : "outline"}
                  >
                    {inv.status}
                  </Badge>
                </td>
                <td className="py-2.5 text-right font-mono tabular-nums">
                  ${usd(inv.amountUsd).toFixed(2)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

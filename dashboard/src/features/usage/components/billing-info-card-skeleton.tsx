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
import { Skeleton } from "@/common/components/ui/skeleton";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/** The card's static header, shared by the ready card and its skeleton. */
export function BillingInfoCardHeader() {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("usage.billingInfoTitle")}</CardTitle>
      <CardDescription>{t("usage.billingInfoDescription")}</CardDescription>
    </CardHeader>
  );
}

/**
 * BillingInfoCard's pending shape (w6/m628). The header and the four row
 * labels are static copy, so they render as the ready card's text; each value
 * is a one-line bar, as an unset part reads on one line, and the action is
 * "Add billing info", what a workspace that never set any sees. A workspace
 * that has no billing account yet adds a note under the rows once it loads.
 *
 * Skeleton-only, so the route's pending component can import it without
 * pulling the page's chunk up front.
 */
export function BillingInfoCardSkeleton() {
  const { t } = useTranslations();
  return (
    <Card data-testid="billing-info-skeleton">
      <BillingInfoCardHeader />
      <BillingInfoCardBody
        details={
          <BillingInfoDetails
            billTo={<Skeleton className="h-5 w-36 max-w-full" />}
            companyName={<Skeleton className="h-5 w-28 max-w-full" />}
            address={<Skeleton className="h-5 w-48 max-w-full" />}
            taxId={<Skeleton className="h-5 w-32 max-w-full" />}
          />
        }
        note={null}
        action={
          <ReservedButton variant="outline" size="sm">
            {t("usage.billingInfoAdd")}
          </ReservedButton>
        }
      />
    </Card>
  );
}

/**
 * The card's body layout, shared by the ready card and its skeleton so the
 * two cannot drift: the details with a note under them, and the action beside
 * them, under them on a phone.
 */
export function BillingInfoCardBody({
  details,
  note,
  action,
}: {
  details: React.ReactNode;
  note: React.ReactNode;
  action: React.ReactNode;
}) {
  return (
    <CardContent className="flex flex-col items-start gap-4 sm:flex-row sm:justify-between">
      <div className="w-full min-w-0 space-y-3 sm:flex-1">
        {details}
        {note ? <p className="text-xs text-muted-foreground">{note}</p> : null}
      </div>
      {action}
    </CardContent>
  );
}

/**
 * The four rows invoices take from the billing info, labelled, label above
 * value on a phone and beside it from `sm` up. Shared by the ready card and
 * its skeleton.
 */
export function BillingInfoDetails({
  billTo,
  companyName,
  address,
  taxId,
}: {
  billTo: React.ReactNode;
  companyName: React.ReactNode;
  address: React.ReactNode;
  taxId: React.ReactNode;
}) {
  const { t } = useTranslations();
  const rows: [string, string, React.ReactNode][] = [
    ["bill-to", t("usage.billingInfoBillTo"), billTo],
    ["company-name", t("usage.billingInfoCompanyName"), companyName],
    ["address", t("usage.billingInfoAddress"), address],
    ["tax-id", t("usage.billingInfoTaxId"), taxId],
  ];
  return (
    <dl className="space-y-2 text-sm">
      {rows.map(([key, label, value]) => (
        <div
          key={key}
          data-billing-info-row={key}
          className="grid gap-0.5 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-4"
        >
          <dt className="text-muted-foreground">{label}</dt>
          <dd className="flex min-h-5 min-w-0 flex-col break-words">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

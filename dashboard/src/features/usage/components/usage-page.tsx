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
import { formatDateISO } from "@/common/lib/format";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { SvgLineChart } from "@/features/metrics/components/svg-line-chart";
import { seriesColor } from "@/features/metrics/components/chart-layout";
import { periodFor, periodLabel } from "@/features/usage/lib/period";
import { Badge } from "@/common/components/ui/badge.tsx";
import {
  useUsage,
  type ServiceUsage,
  type UsageRow,
  type EstimatedCost,
  type Billing,
  type BillingCredits,
} from "../hooks/use-usage";
import { useUsageTrend, type TrendPoint } from "../hooks/use-usage-trend";
import { WorkspaceResourceCaps } from "./resource-caps";
import { BillingOnboardingCard } from "./billing-onboarding";

// --- unit conversion helpers ---

function instanceSecondsToHours(seconds: number): string {
  return (seconds / 3600).toFixed(2);
}

function egressBytesToDisplay(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`;
  return `${mb.toFixed(2)} MB`;
}

function buildSecondsToMinutes(seconds: number): string {
  return (seconds / 60).toFixed(1);
}

function storageGBSecondsToGBHours(gbSeconds: number): string {
  return (gbSeconds / 3600).toFixed(2);
}

// --- month picker helpers ---

/** Build options for `count` months starting `startFrom` months back (oldest first in JSX, but here newest-first). */
function buildMonthOptions(
  count: number,
  startFrom = 0,
): { value: string; label: string }[] {
  return Array.from({ length: count }, (_, i) => {
    const p = periodFor(i + startFrom);
    return { value: p, label: periodLabel(p) };
  });
}

// --- per-section data extraction ---

interface ComputeRow {
  serviceId: string;
  serviceName: string;
  resourceKind: string;
  tier: string;
  total: number;
}

interface ServiceTotalRow {
  serviceId: string;
  serviceName: string;
  total: number;
}

interface StorageRow extends ServiceTotalRow {
  resourceKind: string;
}

function extractRows<T>(
  services: ServiceUsage[],
  kind: string,
  pick: (svc: ServiceUsage, r: UsageRow) => T,
): T[] {
  return services.flatMap((svc) =>
    svc.rows.filter((r) => r.kind === kind).map((r) => pick(svc, r)),
  );
}

/** Display name with id fallback for resources that no longer resolve to one. */
function serviceLabel(r: { serviceId: string; serviceName: string }): string {
  return r.serviceName || r.serviceId;
}

// --- section components ---

function SkeletonTable({ cols }: { cols: number }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          {Array.from({ length: cols }).map((_, i) => (
            <TableHead key={i}>
              <Skeleton className="h-4 w-20" />
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {Array.from({ length: 3 }).map((_, i) => (
          <TableRow key={i}>
            {Array.from({ length: cols }).map((_, j) => (
              <TableCell key={j}>
                <Skeleton className="h-4 w-24" />
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

/**
 * One usage table: a card, a loading skeleton, an empty state, and a table with
 * a trailing total row. The four usage categories differ only in their label
 * columns and how the numeric total is formatted, so they share this shape
 * instead of four near-identical copies.
 */
function UsageTableSection<T extends { total: number }>({
  title,
  description,
  rows,
  loading,
  rowKey,
  labelColumns,
  valueHeader,
  format,
}: {
  title: string;
  description: string;
  rows: T[];
  loading: boolean;
  rowKey: (row: T) => string;
  labelColumns: { header: string; cell: (row: T) => React.ReactNode }[];
  valueHeader: string;
  format: (total: number) => string;
}) {
  const { t } = useTranslations();
  const total = rows.reduce((sum, r) => sum + r.total, 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t(title)}</CardTitle>
        <CardDescription>{t(description)}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && rows.length === 0 ? (
          <SkeletonTable cols={labelColumns.length + 1} />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                {labelColumns.map((col) => (
                  <TableHead key={col.header}>{t(col.header)}</TableHead>
                ))}
                <TableHead className="text-right tabular-nums">
                  {t(valueHeader)}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={rowKey(r)}>
                  {labelColumns.map((col) => (
                    <TableCell key={col.header}>{col.cell(r)}</TableCell>
                  ))}
                  <TableCell className="text-right tabular-nums">
                    {format(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell
                  colSpan={
                    labelColumns.length > 1 ? labelColumns.length : undefined
                  }
                >
                  {t("usage.totalRow")}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {format(total)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function ComputeSection({
  rows,
  loading,
}: {
  rows: ComputeRow[];
  loading: boolean;
}) {
  return (
    <UsageTableSection
      title="usage.computeTitle"
      description="usage.computeDescription"
      rows={rows}
      loading={loading}
      rowKey={(r) => `${r.resourceKind}:${r.serviceId}:${r.tier}`}
      labelColumns={[
        {
          header: "usage.colService",
          cell: (r) => <span className="font-medium">{serviceLabel(r)}</span>,
        },
        {
          header: "usage.colKind",
          cell: (r) => (
            <span className="text-muted-foreground">
              {r.resourceKind || "service"}
            </span>
          ),
        },
        {
          header: "usage.colPlan",
          cell: (r) => (
            <span className="text-muted-foreground capitalize">
              {r.tier || "—"}
            </span>
          ),
        },
      ]}
      valueHeader="usage.colHours"
      format={instanceSecondsToHours}
    />
  );
}

function BandwidthSection({
  rows,
  loading,
}: {
  rows: ServiceTotalRow[];
  loading: boolean;
}) {
  return (
    <UsageTableSection
      title="usage.bandwidthTitle"
      description="usage.bandwidthDescription"
      rows={rows}
      loading={loading}
      rowKey={(r) => r.serviceId}
      labelColumns={[
        {
          header: "usage.colService",
          cell: (r) => <span className="font-medium">{serviceLabel(r)}</span>,
        },
      ]}
      valueHeader="usage.colBandwidth"
      format={egressBytesToDisplay}
    />
  );
}

function BuildSection({
  rows,
  loading,
}: {
  rows: ServiceTotalRow[];
  loading: boolean;
}) {
  return (
    <UsageTableSection
      title="usage.buildTitle"
      description="usage.buildDescription"
      rows={rows}
      loading={loading}
      rowKey={(r) => r.serviceId}
      labelColumns={[
        {
          header: "usage.colService",
          cell: (r) => <span className="font-medium">{serviceLabel(r)}</span>,
        },
      ]}
      valueHeader="usage.colMinutes"
      format={buildSecondsToMinutes}
    />
  );
}

function StorageSection({
  rows,
  loading,
}: {
  rows: StorageRow[];
  loading: boolean;
}) {
  return (
    <UsageTableSection
      title="usage.storageTitle"
      description="usage.storageDescription"
      rows={rows}
      loading={loading}
      rowKey={(r) => `${r.resourceKind}:${r.serviceId}`}
      labelColumns={[
        {
          header: "usage.colService",
          cell: (r) => <span className="font-medium">{serviceLabel(r)}</span>,
        },
        {
          header: "usage.colKind",
          cell: (r) => (
            <span className="text-muted-foreground">{r.resourceKind}</span>
          ),
        },
      ]}
      valueHeader="usage.colGBHours"
      format={storageGBSecondsToGBHours}
    />
  );
}

// --- estimated cost section ---

function EstimatedCostSection({
  estimatedCost,
  loading,
}: {
  estimatedCost: EstimatedCost | null;
  loading: boolean;
}) {
  const { t } = useTranslations();
  const hasCost = estimatedCost && estimatedCost.meters.length > 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.estimatedCostTitle")}</CardTitle>
        <CardDescription>{t("usage.estimatedCostDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && !estimatedCost ? (
          <SkeletonTable cols={4} />
        ) : !hasCost ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.estimatedCostUnavailable")}
          </p>
        ) : (
          <>
            <div className="mb-4 flex items-baseline gap-2">
              <span className="text-2xl font-semibold tabular-nums">
                ${estimatedCost.totalUsd}
              </span>
              <span className="text-xs text-muted-foreground">
                {t("usage.estimatedCostNote")}
              </span>
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("usage.colMeter")}</TableHead>
                  <TableHead>{t("usage.colPlan")}</TableHead>
                  <TableHead>{t("usage.colKind")}</TableHead>
                  <TableHead className="text-right tabular-nums">
                    {t("usage.colEstimate")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {estimatedCost.meters.map((m, i) => (
                  <TableRow key={i}>
                    <TableCell className="font-medium">{m.kind}</TableCell>
                    <TableCell className="text-muted-foreground capitalize">
                      {m.tier || "—"}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {m.resourceKind || "—"}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      ${m.costUsd}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </>
        )}
      </CardContent>
    </Card>
  );
}

// --- credits section (remaining Stripe billing credit, w5/m70) ---

/** Display-side USD math on backend-normalized "12.34" strings. */
function usd(amount: string): number {
  const n = Number.parseFloat(amount);
  return Number.isFinite(n) ? n : 0;
}

// CreditsSection renders only when the workspace holds credit (the backend
// omits the block at zero balance) — no empty-state card for everyone else.
// Values come from Stripe's credit APIs, never derived from the invoice
// preview. The card also carries the ADR046 clarification: credit pays
// invoices first, but a payment method is still required.
function CreditsSection({ credits }: { credits: BillingCredits | null }) {
  const { t } = useTranslations();
  if (!credits) return null;
  const expiring = credits.grants.find((g) => g.expiresAt !== "");
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.creditsTitle")}</CardTitle>
        <CardDescription>{t("usage.creditsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-semibold tabular-nums">
            ${credits.availableUsd}
          </span>
          {expiring && (
            <span className="text-xs text-muted-foreground">
              {t("usage.creditsExpiryNote", {
                amount: expiring.remainingUsd,
                date: formatDateISO(expiring.expiresAt) ?? expiring.expiresAt,
              })}
            </span>
          )}
        </div>
        <p className="mt-2 text-xs text-muted-foreground">
          {t("usage.creditsCardStillRequired")}
        </p>
      </CardContent>
    </Card>
  );
}

// --- current spend section (real Stripe billing, m48/m50) ---

// CurrentSpendSection shows the workspace's REAL billing — the current-period
// Stripe invoice preview plus finalized invoices — visually distinct
// from the advisory estimate above (an "Invoice" badge, not "estimate only").
// It renders nothing when there is no real billing (no Subscription, Mode-A
// exclusion, billing off, or a degraded read): the estimate card stands alone,
// so an estimate-only workspace never sees a broken or empty billing card.
function CurrentSpendSection({ billing }: { billing: Billing | null }) {
  const { t } = useTranslations();
  if (!billing || (!billing.currentCost && billing.invoices.length === 0)) {
    return null;
  }
  const cost = billing.currentCost ? usd(billing.currentCost.amountUsd) : 0;
  const available = billing.credits ? usd(billing.credits.availableUsd) : 0;
  const applied = Math.min(available, cost).toFixed(2);
  const due = Math.max(0, cost - available).toFixed(2);
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>{t("usage.currentSpendTitle")}</CardTitle>
          <Badge variant="default">{t("usage.currentSpendBadge")}</Badge>
        </div>
        <CardDescription>{t("usage.currentSpendDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {billing.currentCost && (
          <div className="mb-4">
            <div className="flex items-baseline gap-2">
              <span className="text-2xl font-semibold tabular-nums">
                ${billing.currentCost.amountUsd}
              </span>
              <span className="text-xs text-muted-foreground">
                {t("usage.currentSpendNote")}
              </span>
            </div>
            {billing.credits && (
              <p className="mt-1 text-sm text-muted-foreground tabular-nums">
                {t("usage.creditsAppliedLine", { applied, due })}
              </p>
            )}
          </div>
        )}
        {billing.invoices.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colInvoicePeriod")}</TableHead>
                <TableHead>{t("usage.colInvoiceStatus")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colInvoiceAmount")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {billing.invoices.map((inv) => (
                <TableRow key={inv.id}>
                  <TableCell className="font-medium tabular-nums">
                    {inv.periodStart ? inv.periodStart.slice(0, 10) : "—"}
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{inv.status}</Badge>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    ${inv.amountUsd}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

// --- trend section ---

function scaledSeries(
  points: TrendPoint[],
  key: keyof TrendPoint,
  divisor: number,
) {
  return [
    {
      color: seriesColor(0),
      points: points.map((p) => ({
        timestamp: p.timestamp,
        value: (p[key] as number) / divisor,
      })),
    },
  ];
}

function TrendSection({
  points,
  loading,
}: {
  points: TrendPoint[];
  loading: boolean;
}) {
  const { t } = useTranslations();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.trendTitle")}</CardTitle>
        <CardDescription>{t("usage.trendDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-[180px] w-full rounded-md" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-4">
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">
                {t("usage.computeTitle")}
              </p>
              <SvgLineChart
                unit="h"
                series={scaledSeries(points, "compute", 3600)}
              />
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">
                {t("usage.bandwidthTitle")}
              </p>
              <SvgLineChart
                unit="MB"
                series={scaledSeries(points, "bandwidth", 1024 * 1024)}
              />
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">
                {t("usage.buildTitle")}
              </p>
              <SvgLineChart
                unit="min"
                series={scaledSeries(points, "build", 60)}
              />
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">
                {t("usage.storageTitle")}
              </p>
              <SvgLineChart
                unit="GB-h"
                series={scaledSeries(points, "storage", 3600)}
              />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// --- page ---

const MONTH_OPTIONS = buildMonthOptions(5, 1);
// Sentinel value for Radix Select (can't use empty string).
const CURRENT_MONTH_SENTINEL = "__current__";

export function UsagePage() {
  const { t } = useTranslations();
  const [selectedPeriod, setSelectedPeriod] = useState<string>(
    CURRENT_MONTH_SENTINEL,
  );
  const period =
    selectedPeriod === CURRENT_MONTH_SENTINEL ? undefined : selectedPeriod;

  const { summary, loading, error } = useUsage(period);
  const { points: trendPoints, loading: trendLoading } = useUsageTrend();

  const services = summary?.services ?? [];
  const computeRows = extractRows<ComputeRow>(
    services,
    "instance_seconds",
    (svc, r) => ({
      serviceId: svc.serviceId,
      serviceName: svc.serviceName,
      resourceKind: svc.resourceKind,
      tier: r.tier,
      total: r.total,
    }),
  );
  const bandwidthRows = extractRows<ServiceTotalRow>(
    services,
    "egress_bytes",
    (svc, r) => ({
      serviceId: svc.serviceId,
      serviceName: svc.serviceName,
      total: r.total,
    }),
  );
  const buildRows = extractRows<ServiceTotalRow>(
    services,
    "build_seconds",
    (svc, r) => ({
      serviceId: svc.serviceId,
      serviceName: svc.serviceName,
      total: r.total,
    }),
  );
  const storageRows = extractRows<StorageRow>(
    services,
    "storage_gb_seconds",
    (svc, r) => ({
      serviceId: svc.serviceId,
      serviceName: svc.serviceName,
      resourceKind: svc.resourceKind,
      total: r.total,
    }),
  );

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-xl font-semibold">{t("usage.pageTitle")}</h1>
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

          {error && !summary && (
            <Alert variant="destructive">
              <AlertTitle>{t("usage.errorTitle")}</AlertTitle>
              <AlertDescription>{error.message}</AlertDescription>
            </Alert>
          )}

          {/* Billing first, usage detail second — Render's workspace billing
              page order, adopted with the w5/m70 rename. */}
          <h2 className="text-base font-semibold">
            {t("usage.sectionBilling")}
          </h2>
          <CreditsSection credits={summary?.billing?.credits ?? null} />
          <BillingOnboardingCard />
          <CurrentSpendSection billing={summary?.billing ?? null} />
          <EstimatedCostSection
            estimatedCost={summary?.estimatedCost ?? null}
            loading={loading}
          />

          <h2 className="text-base font-semibold">{t("usage.sectionUsage")}</h2>
          <WorkspaceResourceCaps />
          <ComputeSection rows={computeRows} loading={loading} />
          <BandwidthSection rows={bandwidthRows} loading={loading} />
          <BuildSection rows={buildRows} loading={loading} />
          <StorageSection rows={storageRows} loading={loading} />
          <TrendSection points={trendPoints} loading={trendLoading} />
        </div>
      </div>
    </DashboardLayout>
  );
}

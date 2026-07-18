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
import {
  useUsage,
  type ServiceUsage,
  type UsageRow,
  type EstimatedCost,
} from "../hooks/use-usage";
import { useUsageTrend, type TrendPoint } from "../hooks/use-usage-trend";
import { WorkspaceResourceCaps } from "./resource-caps";

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

function ComputeSection({
  rows,
  loading,
}: {
  rows: ComputeRow[];
  loading: boolean;
}) {
  const { t } = useTranslations();
  const totalSeconds = rows.reduce((s, r) => s + r.total, 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.computeTitle")}</CardTitle>
        <CardDescription>{t("usage.computeDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && rows.length === 0 ? (
          <SkeletonTable cols={4} />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colService")}</TableHead>
                <TableHead>{t("usage.colKind")}</TableHead>
                <TableHead>{t("usage.colPlan")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colHours")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={`${r.resourceKind}:${r.serviceId}:${r.tier}`}>
                  <TableCell className="font-medium">
                    {serviceLabel(r)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {r.resourceKind || "service"}
                  </TableCell>
                  <TableCell className="text-muted-foreground capitalize">
                    {r.tier || "—"}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {instanceSecondsToHours(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell colSpan={3}>{t("usage.totalRow")}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {instanceSecondsToHours(totalSeconds)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function BandwidthSection({
  rows,
  loading,
}: {
  rows: ServiceTotalRow[];
  loading: boolean;
}) {
  const { t } = useTranslations();
  const totalBytes = rows.reduce((s, r) => s + r.total, 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.bandwidthTitle")}</CardTitle>
        <CardDescription>{t("usage.bandwidthDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && rows.length === 0 ? (
          <SkeletonTable cols={2} />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colService")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colBandwidth")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.serviceId}>
                  <TableCell className="font-medium">
                    {serviceLabel(r)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {egressBytesToDisplay(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell>{t("usage.totalRow")}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {egressBytesToDisplay(totalBytes)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function BuildSection({
  rows,
  loading,
}: {
  rows: ServiceTotalRow[];
  loading: boolean;
}) {
  const { t } = useTranslations();
  const totalSeconds = rows.reduce((s, r) => s + r.total, 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.buildTitle")}</CardTitle>
        <CardDescription>{t("usage.buildDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && rows.length === 0 ? (
          <SkeletonTable cols={2} />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colService")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colMinutes")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.serviceId}>
                  <TableCell className="font-medium">
                    {serviceLabel(r)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {buildSecondsToMinutes(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell>{t("usage.totalRow")}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {buildSecondsToMinutes(totalSeconds)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function StorageSection({
  rows,
  loading,
}: {
  rows: StorageRow[];
  loading: boolean;
}) {
  const { t } = useTranslations();
  const totalGBSeconds = rows.reduce((s, r) => s + r.total, 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("usage.storageTitle")}</CardTitle>
        <CardDescription>{t("usage.storageDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {loading && rows.length === 0 ? (
          <SkeletonTable cols={3} />
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("usage.empty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("usage.colService")}</TableHead>
                <TableHead>{t("usage.colKind")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colGBHours")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={`${r.resourceKind}:${r.serviceId}`}>
                  <TableCell className="font-medium">
                    {serviceLabel(r)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {r.resourceKind}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {storageGBSecondsToGBHours(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell colSpan={2}>{t("usage.totalRow")}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {storageGBSecondsToGBHours(totalGBSeconds)}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
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

          <WorkspaceResourceCaps />

          <ComputeSection rows={computeRows} loading={loading} />
          <BandwidthSection rows={bandwidthRows} loading={loading} />
          <BuildSection rows={buildRows} loading={loading} />
          <StorageSection rows={storageRows} loading={loading} />
          <EstimatedCostSection
            estimatedCost={summary?.estimatedCost ?? null}
            loading={loading}
          />
          <TrendSection points={trendPoints} loading={trendLoading} />
        </div>
      </div>
    </DashboardLayout>
  );
}

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
import { useUsage, type ServiceUsage, type UsageRow } from "../hooks/use-usage";

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

// --- per-section data extraction ---

interface ComputeRow {
  serviceId: string;
  tier: string;
  total: number;
}

interface BandwidthRow {
  serviceId: string;
  total: number;
}

interface BuildRow {
  serviceId: string;
  total: number;
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
                <TableHead>{t("usage.colPlan")}</TableHead>
                <TableHead className="text-right tabular-nums">
                  {t("usage.colHours")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={`${r.serviceId}:${r.tier}`}>
                  <TableCell className="font-medium">{r.serviceId}</TableCell>
                  <TableCell className="text-muted-foreground capitalize">
                    {r.tier || "—"}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {instanceSecondsToHours(r.total)}
                  </TableCell>
                </TableRow>
              ))}
              <TableRow className="border-t-2 font-semibold">
                <TableCell colSpan={2}>{t("usage.totalRow")}</TableCell>
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
  rows: BandwidthRow[];
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
                  <TableCell className="font-medium">{r.serviceId}</TableCell>
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
  rows: BuildRow[];
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
                  <TableCell className="font-medium">{r.serviceId}</TableCell>
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

// --- page ---

export function UsagePage() {
  const { t } = useTranslations();
  const { summary, loading, error } = useUsage();

  const services = summary?.services ?? [];
  const computeRows = extractRows<ComputeRow>(services, "instance_seconds", (svc, r) => ({
    serviceId: svc.serviceId,
    tier: r.tier,
    total: r.total,
  }));
  const bandwidthRows = extractRows<BandwidthRow>(services, "egress_bytes", (svc, r) => ({
    serviceId: svc.serviceId,
    total: r.total,
  }));
  const buildRows = extractRows<BuildRow>(services, "build_seconds", (svc, r) => ({
    serviceId: svc.serviceId,
    total: r.total,
  }));

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div>
            <h1 className="text-xl font-semibold">{t("usage.pageTitle")}</h1>
            <p className="text-sm text-muted-foreground">
              {t("usage.pageSubtitle")}
            </p>
          </div>

          {error && !summary && (
            <Alert variant="destructive">
              <AlertTitle>{t("usage.errorTitle")}</AlertTitle>
              <AlertDescription>{error.message}</AlertDescription>
            </Alert>
          )}

          <ComputeSection rows={computeRows} loading={loading} />
          <BandwidthSection rows={bandwidthRows} loading={loading} />
          <BuildSection rows={buildRows} loading={loading} />
        </div>
      </div>
    </DashboardLayout>
  );
}

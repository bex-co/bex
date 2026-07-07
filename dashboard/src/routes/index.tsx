import { createFileRoute, Link } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import { Badge } from "@/common/components/ui/badge.tsx";
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
import { Alert, AlertDescription, AlertTitle } from "@/common/components/ui/alert.tsx";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { useServices } from "@/features/services/hooks/use-services";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { deriveStatus, computeStats } from "@/features/services/lib/status";
import { formatRelativeAge } from "@/features/services/lib/format";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import type {
  ServiceView,
  ServiceStatusKey,
  LifecycleAction,
} from "@/features/services/types";

export const Route = createFileRoute("/")({
  component: HomePage,
  beforeLoad: requireAuth("/"),
  head: () => ({
    meta: [{ title: "bex dashboard" }],
  }),
});

const STATUS_LABEL: Record<ServiceStatusKey, keyof typeof en> = {
  running: "services.statusRunning",
  suspended: "services.statusSuspended",
  hibernated: "services.statusHibernated",
  pending: "services.statusPending",
  building: "services.statusBuilding",
  deploying: "services.statusDeploying",
  failed: "services.statusFailed",
  unknown: "services.statusUnknown",
};

export function HomePage() {
  const { t } = useTranslations();
  const { services, loading, error, refetch } = useServices();
  const { pending, run } = useServiceLifecycle({ refetch });

  const stats = computeStats(services);
  const serviceStats = [
    { labelKey: "services.statTotal", value: stats.total },
    { labelKey: "services.statRunning", value: stats.running },
    { labelKey: "services.statSuspended", value: stats.suspended },
  ] as const;

  // Only treat loading/error as page-level states while there's nothing to show;
  // a transient poll error or background refetch must not blank an existing list.
  const showSkeleton = loading && services.length === 0;
  const showError = !loading && error && services.length === 0;
  const showEmpty = !loading && !error && services.length === 0;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {serviceStats.map((stat) => (
              <Card key={stat.labelKey}>
                <CardHeader>
                  <CardDescription>{t(stat.labelKey)}</CardDescription>
                  <CardTitle className="text-2xl tabular-nums">
                    {stat.value}
                  </CardTitle>
                </CardHeader>
              </Card>
            ))}
          </div>
          <Card>
            <CardHeader>
              <CardTitle>{t("services.cardTitle")}</CardTitle>
            </CardHeader>
            <CardContent>
              {showError ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("services.errorTitle")}</AlertTitle>
                  <AlertDescription>
                    {t("services.errorBody")}
                  </AlertDescription>
                </Alert>
              ) : showEmpty ? (
                <div className="py-10 text-center">
                  <p className="font-medium">{t("services.emptyTitle")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("services.emptyBody")}
                  </p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("services.colName")}</TableHead>
                      <TableHead>{t("services.colStatus")}</TableHead>
                      <TableHead className="text-right tabular-nums">
                        {t("services.colInstances")}
                      </TableHead>
                      <TableHead>{t("services.colRevision")}</TableHead>
                      <TableHead>{t("services.colCreated")}</TableHead>
                      <TableHead>{t("services.colUrl")}</TableHead>
                      <TableHead className="w-0 text-right">
                        <span className="sr-only">
                          {t("services.colActions")}
                        </span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {showSkeleton
                      ? Array.from({ length: 3 }).map((_, i) => (
                          <ServiceSkeletonRow key={i} />
                        ))
                      : services.map((service) => (
                          <ServiceRow
                            key={service.id}
                            service={service}
                            pending={
                              pending?.id === service.id
                                ? pending.action
                                : null
                            }
                            onRun={run}
                            statusLabel={t(
                              STATUS_LABEL[deriveStatus(service).key],
                            )}
                          />
                        ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}

function ServiceRow({
  service,
  pending,
  onRun,
  statusLabel,
}: {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun: (action: LifecycleAction, service: ServiceView) => void;
  statusLabel: string;
}) {
  const status = deriveStatus(service);
  return (
    <TableRow>
      <TableCell className="font-medium">
        <Link
          to="/services/$serviceId/metrics"
          params={{ serviceId: service.id }}
          className="hover:underline"
        >
          {service.name}
        </Link>
      </TableCell>
      <TableCell>
        <Badge variant={status.variant}>{statusLabel}</Badge>
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {service.replicas ?? "—"}
      </TableCell>
      <TableCell className="font-mono text-xs text-muted-foreground">
        {service.revision || "—"}
      </TableCell>
      <TableCell className="tabular-nums text-muted-foreground">
        {formatRelativeAge(service.createdAt)}
      </TableCell>
      <TableCell className="text-muted-foreground">
        {service.url ?? "—"}
      </TableCell>
      <TableCell className="text-right">
        <ServiceRowActions
          service={service}
          pending={pending}
          onRun={onRun}
        />
      </TableCell>
    </TableRow>
  );
}

function ServiceSkeletonRow() {
  return (
    <TableRow>
      <TableCell>
        <Skeleton className="h-4 w-32" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-5 w-16 rounded-md" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto h-4 w-6" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-16" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-10" />
      </TableCell>
      <TableCell>
        <Skeleton className="h-4 w-48" />
      </TableCell>
      <TableCell className="text-right">
        <Skeleton className="ml-auto size-8 rounded-md" />
      </TableCell>
    </TableRow>
  );
}

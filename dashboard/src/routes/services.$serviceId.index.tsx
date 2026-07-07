import { createFileRoute } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { EmptyState } from "@/common/components/empty-state";
import { useServer } from "@/features/services/hooks/use-server";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { formatRelativeAge } from "@/features/services/lib/format";
import type { ServiceView } from "@/features/services/types";

export const Route = createFileRoute("/services/$serviceId/")({
  component: RouteComponent,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · bex dashboard` }],
  }),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceOverviewPage serviceId={serviceId} />;
}

export function ServiceOverviewPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { service, loading, error } = useServer(serviceId);

  // Page-level states only while there's nothing to show — a background refetch
  // must not blank an already-rendered panel (mirrors the list route).
  if (!service && loading) return <OverviewSkeleton />;
  if (!service && error) {
    return (
      <Alert variant="destructive">
        <AlertTitle>{t("services.errorTitle")}</AlertTitle>
        <AlertDescription>{t("services.errorBody")}</AlertDescription>
      </Alert>
    );
  }
  if (!service) {
    return (
      <EmptyState
        iconName="SearchX"
        title={t("services.notFoundTitle")}
        description={t("services.notFoundBody", { name: serviceId })}
      />
    );
  }

  return <OverviewPanel service={service} />;
}

function OverviewPanel({ service }: { service: ServiceView }) {
  const { t } = useTranslations();

  const rows: { label: string; value: React.ReactNode }[] = [
    {
      label: t("services.colStatus"),
      value: <ServiceStatusBadge service={service} />,
    },
    {
      label: t("services.overviewPhase"),
      value: service.phase || "—",
    },
    {
      label: t("services.colUrl"),
      value: service.url ? (
        <a
          href={service.url}
          target="_blank"
          rel="noreferrer"
          className="text-primary hover:underline"
        >
          {service.url}
        </a>
      ) : (
        "—"
      ),
    },
    {
      label: t("services.colInstances"),
      value: <span className="tabular-nums">{service.replicas ?? "—"}</span>,
    },
    {
      label: t("services.colRevision"),
      value: <span className="font-mono text-xs">{service.revision || "—"}</span>,
    },
    {
      label: t("services.colCreated"),
      value: (
        <span className="tabular-nums">{formatRelativeAge(service.createdAt)}</span>
      ),
    },
    {
      label: t("services.overviewSuspended"),
      value: service.suspended
        ? t("services.overviewYes")
        : t("services.overviewNo"),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.navOverview")}</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
          {rows.map((row) => (
            <div key={row.label} className="space-y-1">
              <dt className="text-sm text-muted-foreground">{row.label}</dt>
              <dd className="text-sm font-medium break-all">{row.value}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

function OverviewSkeleton() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <Skeleton className="h-5 w-24" />
        </CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="space-y-1">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-40" />
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils.ts";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useDatabase } from "@/features/databases/hooks/use-database";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { RecoveryPanel } from "@/features/databases/components/recovery-panel";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { HAPanel } from "@/features/databases/components/ha-panel";
import { InsightsPanel } from "@/features/databases/components/insights-panel";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import { DatabaseNameSection } from "@/features/databases/components/database-name-section";
import { DatabaseDiskAutoscalingControl } from "@/features/databases/components/database-disk-autoscaling-control";
import { SQLConsole } from "@/features/databases/components/sql-console";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import { PostgresLogViewer } from "@/features/databases/components/postgres-log-viewer";
import type { DatabaseDetailView } from "@/features/databases/types";

export const Route = createFileRoute("/databases/$databaseId")({
  component: DatabaseDetailPage,
  beforeLoad: requireAuth("/databases/$databaseId"),
  validateSearch: (search: Record<string, unknown>): { tab?: "logs" } =>
    search.tab === "logs" ? { tab: "logs" } : {},
  head: ({ params }) => ({
    meta: [{ title: `${params.databaseId} · Databases · bex dashboard` }],
  }),
});

export function DatabaseDetailPage() {
  const { databaseId } = Route.useParams();
  const { tab } = Route.useSearch();
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { database, loading, refetch } = useDatabase(databaseId);
  const lifecycle = useDatabaseLifecycle({ refetch });

  const showNotFound = !loading && !database;

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div className="flex items-center gap-2">
          <h1
            className={cn(
              "truncate text-xl font-semibold",
              !database && "text-muted-foreground",
            )}
          >
            {database?.name ?? databaseId}
          </h1>
          {database ? <DatabaseStatusBadge status={database.status} /> : null}
        </div>
        {database ? (
          <DatabaseRowActions
            database={database}
            onDeleted={() => void navigate({ to: "/" })}
            lifecycle={lifecycle}
          />
        ) : null}
      </div>

      <nav
        aria-label={t("databases.detailNavLabel")}
        className="flex gap-1 border-b px-4 sm:px-6"
      >
        <button
          type="button"
          className={cn(
            "border-b-2 px-3 py-2 text-sm",
            tab !== "logs"
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground",
          )}
          onClick={() => void navigate({ to: ".", search: {} })}
        >
          {t("databases.overviewTab")}
        </button>
        <button
          type="button"
          className={cn(
            "border-b-2 px-3 py-2 text-sm",
            tab === "logs"
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground",
          )}
          onClick={() => void navigate({ to: ".", search: { tab: "logs" } })}
        >
          {t("databases.logsTab")}
        </button>
      </nav>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {showNotFound ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("databases.notFoundTitle")}</p>
              <p className="text-sm text-muted-foreground">
                {t("databases.notFoundBody", { name: databaseId })}
              </p>
            </div>
          ) : database && tab === "logs" ? (
            <PostgresLogViewer resource={database.id} />
          ) : database ? (
            <>
              <MetadataCard
                database={database}
                onChanged={() => void refetch()}
              />
              <DatabaseNameSection
                key={`name-${database.name}`}
                database={database}
                onChanged={() => void refetch()}
              />
              <ConnectionInfoPanel id={database.id} />
              <SQLConsole key={`sql-${database.id}`} id={database.id} />
              <HAPanel database={database} refetch={refetch} />
              <DatastoreMetricsPanel
                kind="database"
                resource={database.id}
                highAvailabilityEnabled={database.highAvailabilityEnabled}
                diskHeaderExtra={
                  <DatabaseDiskAutoscalingControl
                    database={database}
                    onChanged={() => void refetch()}
                  />
                }
              />
              <DatabasePlanSection
                database={database}
                onChanged={() => void refetch()}
              />
              <InsightsPanel id={database.id} />
              <RecoveryPanel id={database.id} />
              <AccessControlPanel id={database.id} />
            </>
          ) : (
            <Skeleton className="h-64 w-full" />
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function MetadataCard({
  database,
  onChanged,
}: {
  database: DatabaseDetailView;
  onChanged: () => void;
}) {
  const { t } = useTranslations();
  return (
    <MetadataList
      title={t("databases.metaTitle")}
      rows={[
        { label: t("databases.metaStatus"), value: database.status || "—" },
        { label: t("databases.metaPlan"), value: database.plan ?? "—" },
        {
          label: t("databases.metaVersion"),
          value: (
            <DatabaseVersionControl database={database} onChanged={onChanged} />
          ),
        },
        {
          label: t("databases.metaDatabaseName"),
          value: database.databaseName ?? "—",
        },
        {
          label: t("databases.metaDatabaseUser"),
          value: database.databaseUser ?? "—",
        },
        {
          label: t("databases.metaStorage"),
          value: database.diskSizeGB ? `${database.diskSizeGB} GB` : "—",
        },
        {
          label: t("databases.metaHighAvailability"),
          value: database.highAvailabilityEnabled
            ? t("databases.yes")
            : t("databases.no"),
        },
        {
          label: t("databases.metaPublic"),
          value: database.public ? t("databases.yes") : t("databases.no"),
        },
        {
          label: t("databases.metaExternalHost"),
          value: database.externalHost ?? "—",
        },
        ...(database.region
          ? [
              {
                label: t("databases.metaRegion"),
                value: database.region,
              },
            ]
          : []),
        {
          label: t("databases.metaCreated"),
          value: formatRelativeAge(database.createdAt),
        },
      ]}
    />
  );
}

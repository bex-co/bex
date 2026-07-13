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
import type { DatabaseDetailView } from "@/features/databases/types";

export const Route = createFileRoute("/databases/$databaseId")({
  component: DatabaseDetailPage,
  beforeLoad: requireAuth("/databases/$databaseId"),
  head: ({ params }) => ({
    meta: [{ title: `${params.databaseId} · Databases · bex dashboard` }],
  }),
});

export function DatabaseDetailPage() {
  const { databaseId } = Route.useParams();
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
            onDeleted={() => void navigate({ to: "/databases" })}
            lifecycle={lifecycle}
          />
        ) : null}
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {showNotFound ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("databases.notFoundTitle")}</p>
              <p className="text-sm text-muted-foreground">
                {t("databases.notFoundBody", { name: databaseId })}
              </p>
            </div>
          ) : database ? (
            <>
              <MetadataCard database={database} />
              <ConnectionInfoPanel id={database.id} />
              <HAPanel database={database} refetch={refetch} />
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

function MetadataCard({ database }: { database: DatabaseDetailView }) {
  const { t } = useTranslations();
  return (
    <MetadataList
      title={t("databases.metaTitle")}
      rows={[
        { label: t("databases.metaStatus"), value: database.status || "—" },
        { label: t("databases.metaPlan"), value: database.plan ?? "—" },
        {
          label: t("databases.metaVersion"),
          value: database.version ? `PostgreSQL ${database.version}` : "—",
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
        {
          label: t("databases.metaCreated"),
          value: formatRelativeAge(database.createdAt),
        },
      ]}
    />
  );
}

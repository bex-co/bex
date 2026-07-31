import {
  createFileRoute,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import {
  CardSkeleton,
  MetadataListSkeleton,
} from "@/common/components/detail-skeletons";
import { cn } from "@/common/lib/utils/utils.ts";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useDatabase } from "@/features/databases/hooks/use-database";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { RecoveryPanel } from "@/features/databases/components/recovery-panel";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { HAPanel } from "@/features/databases/components/ha-panel";
import { InsightsPanel } from "@/features/databases/components/insights-panel";
import { DatabasePlanSection } from "@/features/databases/components/database-plan-section";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import { DatabaseNameRow } from "@/features/databases/components/database-name-row";
import { DatabaseDiskAutoscalingControl } from "@/features/databases/components/database-disk-autoscaling-control";
import { SQLConsole } from "@/features/databases/components/sql-console";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import { PostgresLogViewer } from "@/features/databases/components/postgres-log-viewer";
import type { DatabaseDetailView } from "@/features/databases/types";
import { DatabaseDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/databases/$databaseId")({
  component: DatabaseDetailPage,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: DatabaseDetailPage,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  validateSearch: (search: Record<string, unknown>): { tab?: "logs" } =>
    search.tab === "logs" ? { tab: "logs" } : {},
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: DatabaseDocument,
          variables: { id: params.databaseId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.database?.name?.trim() ? data.database : null),
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (database) => [
        database.name,
        translatedText("databases.resourceType"),
      ]),
      match,
    ),
});

export function DatabaseDetailPage() {
  const { databaseId } = Route.useParams();
  const { tab } = Route.useSearch();
  const { t } = useTranslations();
  const navigate = useNavigate();
  const router = useRouter();
  const { database, loading, error, refetch } = useDatabase(databaseId);
  const lifecycle = useDatabaseLifecycle({ refetch });

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted database. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(!loading && !database && !error);
  useLoaderErrorRetry(Route.useLoaderData(), databaseId);
  const showError = !loading && !database && !!error;

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
          {database ? <DatabaseStatusBadge database={database} /> : null}
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
          {showError ? (
            <ResourceLoadError onRetry={() => void refetch()} />
          ) : database && tab === "logs" ? (
            <PostgresLogViewer resource={database.id} />
          ) : database ? (
            <>
              <MetadataCard
                database={database}
                onVersionChanged={() => void refetch()}
                onRenamed={() => void router.invalidate()}
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
              <DatabaseDangerActions
                database={database}
                onDeleted={() => void navigate({ to: "/" })}
                lifecycle={lifecycle}
              />
            </>
          ) : (
            <>
              <MetadataListSkeleton rows={10} />
              <CardSkeleton rows={3} />
              <CardSkeleton rows={4} />
              <CardSkeleton rows={3} />
            </>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function MetadataCard({
  database,
  onVersionChanged,
  onRenamed,
}: {
  database: DatabaseDetailView;
  onVersionChanged: () => void;
  onRenamed: () => void;
}) {
  const { t } = useTranslations();
  return (
    <MetadataList
      title={t("databases.metaTitle")}
      lead={<DatabaseNameRow database={database} onRenamed={onRenamed} />}
      rows={[
        { label: t("databases.metaStatus"), value: database.status || "—" },
        { label: t("databases.metaPlan"), value: database.plan ?? "—" },
        {
          label: t("databases.metaVersion"),
          value: (
            <DatabaseVersionControl
              database={database}
              onChanged={onVersionChanged}
            />
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

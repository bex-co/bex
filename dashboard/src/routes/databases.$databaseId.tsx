import {
  createFileRoute,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { lazy, Suspense } from "react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  DatabaseOverviewSkeleton,
  DatastoreLogsSkeleton,
} from "@/common/components/route-skeletons";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  resourceFailed,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils.ts";
import { RelativeAge } from "@/common/components/relative-time";
import { useDatabase } from "@/features/databases/hooks/use-database";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { DatabaseDangerActions } from "@/features/databases/components/database-danger-actions";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import { DatabaseNameRow } from "@/features/databases/components/database-name-row";
import { DatabaseDetailNavigation } from "@/features/databases/components/database-detail-navigation";
import { DatabaseDiskAutoscalingControl } from "@/features/databases/components/database-disk-autoscaling-control";
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
import { DeferredMount } from "@/common/components/deferred-mount";
import { SECTION_NAVIGATION_STICKY_CLASS } from "@/common/components/section-navigation";

const RecoveryPanel = lazy(() =>
  import("@/features/databases/components/recovery-panel").then((m) => ({
    default: m.RecoveryPanel,
  })),
);
const AccessControlPanel = lazy(() =>
  import("@/features/databases/components/access-control-panel").then((m) => ({
    default: m.AccessControlPanel,
  })),
);
const HAPanel = lazy(() =>
  import("@/features/databases/components/ha-panel").then((m) => ({
    default: m.HAPanel,
  })),
);
const InsightsPanel = lazy(() =>
  import("@/features/databases/components/insights-panel").then((m) => ({
    default: m.InsightsPanel,
  })),
);
const DatabasePlanSection = lazy(() =>
  import("@/features/databases/components/database-plan-section").then((m) => ({
    default: m.DatabasePlanSection,
  })),
);
const SQLConsole = lazy(() =>
  import("@/features/databases/components/sql-console").then((m) => ({
    default: m.SQLConsole,
  })),
);
const DatastoreMetricsPanel = lazy(() =>
  import("@/features/metrics/components/datastore-metrics-panel").then((m) => ({
    default: m.DatastoreMetricsPanel,
  })),
);

export const Route = createFileRoute("/databases/$databaseId")({
  staticData: { chrome: true },
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

function DatabaseDetailPage() {
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
  useNotFoundRedirect(resourceNotFound(database, loading, error));
  useLoaderErrorRetry(Route.useLoaderData(), databaseId);
  const showError = resourceFailed(database, loading, error);

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!database ? "database-detail" : undefined}
      >
        <div
          data-skeleton-region={!database ? "resource-header" : undefined}
          className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6"
        >
          {database ? (
            <div className="flex items-center gap-2">
              <h1 className="truncate text-xl font-semibold">
                {database.name}
              </h1>
              <DatabaseStatusBadge database={database} />
            </div>
          ) : (
            <div
              data-skeleton-region="resource-title"
              className="flex items-center gap-2"
            >
              <Skeleton className="h-6 w-48" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          )}
          {database ? (
            <DatabaseRowActions
              database={database}
              onDeleted={() => void navigate({ to: "/", replace: true })}
              lifecycle={lifecycle}
            />
          ) : (
            <div data-skeleton-region="resource-actions">
              <Skeleton className="size-9" />
            </div>
          )}
        </div>

        <nav
          data-skeleton-region={!database ? "tabs" : undefined}
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

        <div
          data-skeleton-region={!database ? "active-tab" : undefined}
          className="flex-1 overflow-auto p-4 sm:p-6"
        >
          {showError ? (
            <div className="mx-auto w-full max-w-4xl space-y-6">
              <ResourceLoadError onRetry={() => void refetch()} />
            </div>
          ) : database && tab === "logs" ? (
            <div className="mx-auto w-full max-w-4xl space-y-6">
              <PostgresLogViewer resource={database.id} />
            </div>
          ) : database ? (
            <div className="mx-auto grid w-full max-w-6xl items-start gap-6 lg:grid-cols-[minmax(0,1fr)_13rem] lg:gap-10">
              {/* Same right-rail quick nav as the service settings page. */}
              <DatabaseDetailNavigation
                className={SECTION_NAVIGATION_STICKY_CLASS}
              />

              <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
                <section id="metadata" className="scroll-mt-6">
                  <MetadataCard
                    database={database}
                    onVersionChanged={() => void refetch()}
                    onRenamed={() => void router.invalidate()}
                  />
                </section>
                <section id="connection" className="scroll-mt-6">
                  <ConnectionInfoPanel id={database.id} />
                </section>
                <section id="sql-console" className="scroll-mt-6">
                  <DeferredMount hashId="sql-console" minHeight={280}>
                    <Suspense fallback={<CardSkeleton rows={4} />}>
                      <SQLConsole key={`sql-${database.id}`} id={database.id} />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="high-availability" className="scroll-mt-6">
                  <DeferredMount hashId="high-availability" minHeight={160}>
                    <Suspense fallback={<CardSkeleton rows={2} />}>
                      <HAPanel database={database} refetch={refetch} />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="metrics" className="scroll-mt-6">
                  <DeferredMount hashId="metrics" minHeight={320}>
                    <Suspense fallback={<CardSkeleton rows={4} />}>
                      <DatastoreMetricsPanel
                        kind="database"
                        resource={database.id}
                        highAvailabilityEnabled={
                          database.highAvailabilityEnabled
                        }
                        diskHeaderExtra={
                          <DatabaseDiskAutoscalingControl
                            database={database}
                            onChanged={() => void refetch()}
                          />
                        }
                      />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="plan" className="scroll-mt-6">
                  <DeferredMount hashId="plan" minHeight={200}>
                    <Suspense fallback={<CardSkeleton rows={3} />}>
                      <DatabasePlanSection
                        database={database}
                        onChanged={() => void refetch()}
                      />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="insights" className="scroll-mt-6">
                  <DeferredMount hashId="insights" minHeight={360}>
                    <Suspense fallback={<CardSkeleton rows={4} />}>
                      <InsightsPanel id={database.id} />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="recovery" className="scroll-mt-6">
                  <DeferredMount hashId="recovery" minHeight={240}>
                    <Suspense fallback={<CardSkeleton rows={3} />}>
                      <RecoveryPanel id={database.id} />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="access-control" className="scroll-mt-6">
                  <DeferredMount hashId="access-control" minHeight={240}>
                    <Suspense fallback={<CardSkeleton rows={3} />}>
                      <AccessControlPanel id={database.id} />
                    </Suspense>
                  </DeferredMount>
                </section>
                <section id="danger-zone" className="scroll-mt-6">
                  <DatabaseDangerActions
                    database={database}
                    onDeleted={() => void navigate({ to: "/", replace: true })}
                    lifecycle={lifecycle}
                  />
                </section>
              </div>
            </div>
          ) : tab === "logs" ? (
            <DatastoreLogsSkeleton />
          ) : (
            <DatabaseOverviewSkeleton />
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
          value: <RelativeAge value={database.createdAt} />,
        },
      ]}
    />
  );
}

import { PageScroll } from "@/common/components/page-frame";
import {
  Outlet,
  createFileRoute,
  useRouterState,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  DatabaseAppsSkeleton,
  DatabaseInfoSkeleton,
  DatabaseMetricsSkeleton,
  DatabasePlanSkeleton,
  DatabaseRecoverySkeleton,
  DatabaseVersionUpgradeSkeleton,
  DatastoreLogsSkeleton,
} from "@/common/components/route-skeletons";
import {
  ResourceStatePage,
  ResourceStateView,
} from "@/common/components/resource-not-found";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useScrollToHashedCard } from "@/common/hooks/use-scroll-to-hashed-card";
import { resourceReadState } from "@/common/lib/resource-read-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useDatabase } from "@/features/databases/hooks/use-database";
import type { DatabaseDetailView } from "@/features/databases/types";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import {
  DatabaseHeaderActions,
  DatabasePlanBadge,
} from "@/features/databases/components/database-header";
import { DatastoreHeaderActionsSkeleton } from "@/common/components/datastore-connect-menu";
import { DatabaseStorageAlert } from "@/features/databases/components/database-storage-alert";
import {
  DatabaseUpgradeAvailableBanner,
  DatabaseUpgradeCloneBanner,
  DatabaseUpgradeFailedBanner,
} from "@/features/databases/components/database-upgrade-notices";
import { databasePageLabelKey } from "@/features/databases/components/database-nav";
import { DatabaseDetailProvider } from "@/features/databases/lib/database-detail";
import { DatabaseDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  type RouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/databases/$databaseId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms: the layout's chrome and the open page's
  // skeleton without the live <Outlet/>, so the title-loader wait shows the
  // real frame instead of the router-level blank that used to flash white,
  // and never a second live copy of the page (w4/m236, as the service
  // layout does).
  pendingComponent: PendingRouteComponent,
  pendingMs: 0,
  // The pending frame is drawn with the layout: keep it in the layout's chunk
  // instead of the default reference-file placement (vite.config.ts).
  codeSplitGroupings: [
    ["component", "pendingComponent"],
    ["errorComponent"],
    ["notFoundComponent"],
  ],
  beforeLoad: requireAuth(),
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
  // Each page leads this with its own name (databasePageHead).
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (database) => [
        database.name,
        translatedText("databases.resourceType"),
      ]),
      match,
    ),
});

function PendingRouteComponent() {
  return <DatabaseDetailLayout routePending />;
}

function RouteComponent() {
  // A roll-window loader failure re-runs once (w1/m52) so the title recovers.
  const loaderData = Route.useLoaderData();
  useLoaderErrorRetry(loaderData, Route.useParams().databaseId);
  return <DatabaseDetailLayout routeState={loaderData?.state} />;
}

/**
 * A Postgres database's pages share this layout (w3/m641), as Render's do:
 * the header (name, status, plan badge, View docs and Connect, w3/m675), the
 * open page's heading, which is its sidebar entry's label, and an `<Outlet/>`
 * for the page. As on Render, the header has no overflow menu: Suspend,
 * Resume, Restart and Delete are in Info's danger zone, and Logs and Metrics
 * draw no header at all (w3/117). The sidebar is DashboardSidebar's
 * DatabaseSidebar. The layout reads the database once and hands it to its
 * pages through DatabaseDetailProvider.
 */
function DatabaseDetailLayout({
  routePending = false,
  routeState,
}: {
  /**
   * Rendered as the route's pendingComponent while its title loader runs:
   * the same chrome with the page's skeleton, never the live `<Outlet/>`.
   */
  routePending?: boolean;
  /** The route loader's verdict, which alone can tell a refused read from a
   *  missing one (`resourceReadState`). */
  routeState?: RouteResource<unknown>["state"];
}) {
  const { databaseId } = Route.useParams();
  const { t } = useTranslations();
  const { database: loaded, loading, error, refetch } = useDatabase(databaseId);
  const lifecycle = useDatabaseLifecycle({ refetch });
  // Judge the pages' controls in the database's own workspace (w4/641, as
  // w6/091 for services): until it is the selection, the skeleton stays up so
  // no role-gated control paints for the wrong workspace first.
  const adopting = useAdoptResourceWorkspace(
    databaseId,
    routePending ? null : loaded?.ownerId,
  );
  const database = adopting || routePending ? null : loaded;

  // A dead id keeps its URL and draws Render's Page not found; a refused read
  // draws Access denied and any other failure "An error occurred", so an
  // outage never masquerades as a deleted database (w3/m706).
  const readState = routePending
    ? null
    : resourceReadState(loaded, loading, error, routeState);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const pageLabelKey = databasePageLabelKey(pathname, databaseId);
  // From the pathname, so the pending frame leaves the header out too.
  const headerHidden = headerHiddenOn(pageLabelKey);
  useScrollToHashedCard(pathname, database !== null);

  if (readState) {
    return (
      <ResourceStatePage>
        <ResourceStateView state={readState} id={databaseId} />
      </ResourceStatePage>
    );
  }

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!database ? "database-detail" : undefined}
      >
        {headerHidden ? null : (
          <div
            data-skeleton-region={!database ? "resource-header" : undefined}
            className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6"
          >
            {database ? (
              <div className="flex min-w-0 max-w-full items-center gap-2">
                <h1 className="truncate text-xl font-semibold">
                  {database.name}
                </h1>
                <DatabaseStatusBadge database={database} />
                <DatabasePlanBadge database={database} />
              </div>
            ) : (
              <div
                data-skeleton-region="resource-title"
                className="flex min-w-0 max-w-full items-center gap-2"
              >
                {/* The h1's text-xl line box is 28px (w4/m209); a badge is
                    22px. */}
                <Skeleton className="h-7 w-48" />
                <Skeleton className="h-[22px] w-16 rounded-full" />
                <Skeleton className="h-[22px] w-14 rounded-full" />
              </div>
            )}
            {database ? (
              <DatabaseHeaderActions database={database} />
            ) : (
              <DatastoreHeaderActionsSkeleton />
            )}
          </div>
        )}

        <PageScroll
          data-skeleton-region={!database ? "active-page" : undefined}
        >
          <div className="mx-auto w-full max-w-5xl space-y-6">
            {database ? (
              // A clone made for a test upgrade says so above every page,
              // as Render's does (w3/m676).
              <DatabaseUpgradeCloneBanner database={database} />
            ) : null}
            {database && pageLabelKey === "databases.navInfo" ? (
              // Render's Info opens with a newer major's announcement and a
              // failed upgrade's result, above its title (w3/m676).
              <>
                <DatabaseUpgradeAvailableBanner database={database} />
                <DatabaseUpgradeFailedBanner database={database} />
              </>
            ) : null}
            {database && storageAlertAbovePage(pageLabelKey) ? (
              // Render draws its almost-full warning first on Info and Plan,
              // above Info's title (w3/m656).
              <DatabaseStorageAlert database={database} />
            ) : null}
            {pageLabelKey ? (
              // Every page opens with its name, the sidebar's label for it.
              <h2 className="text-lg leading-7 font-semibold tracking-tight">
                {t(pageLabelKey)}
              </h2>
            ) : null}
            {database ? (
              <DatabaseDetailProvider value={{ database, refetch, lifecycle }}>
                <Outlet />
              </DatabaseDetailProvider>
            ) : (
              <DatabasePageSkeleton
                labelKey={pageLabelKey}
                versionUpgrade={isVersionUpgradePage(pathname)}
                database={loaded}
              />
            )}
          </div>
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/** Whether `pathname` is a database's guided version-upgrade page. */
function isVersionUpgradePage(pathname: string): boolean {
  return pathname.replace(/\/+$/, "").endsWith("/version-upgrade");
}

/**
 * The pages Render draws without the database header (w3/117): Logs and
 * Metrics. Render's Postgres layout passes `hideResourceHeader` on its
 * full-screen pages (`index-*.js`, the fluid-layout hook of `CronJob-*.js`:
 * a path with `/logs`, `/metrics` or `/shell`), and its page template then
 * skips the header and the banners under it, giving the log explorer and the
 * charts the page's full height. A signed-in walk on 2026-11-25 confirmed it
 * (w3/106). The sidebar and the breadcrumb still name the database.
 */
function headerHiddenOn(
  labelKey: ReturnType<typeof databasePageLabelKey>,
): boolean {
  return (
    labelKey === "databases.navLogs" || labelKey === "databases.navMetrics"
  );
}

/** The pages whose almost-full warning leads them, above their heading. */
function storageAlertAbovePage(
  labelKey: ReturnType<typeof databasePageLabelKey>,
): boolean {
  return labelKey === "databases.navInfo" || labelKey === "databases.navPlan";
}

/**
 * The open page's own pending frame, the one its route shows on a page
 * switch, while the layout has no database yet. `database` is the cache's
 * copy when there is one (a workspace still being adopted).
 */
function DatabasePageSkeleton({
  labelKey,
  versionUpgrade,
  database,
}: {
  labelKey: ReturnType<typeof databasePageLabelKey>;
  /** The guided version-upgrade page, which has no sidebar entry. */
  versionUpgrade: boolean;
  database: DatabaseDetailView | null;
}) {
  if (versionUpgrade) {
    return <DatabaseVersionUpgradeSkeleton database={database} />;
  }
  switch (labelKey) {
    case "databases.navLogs":
      return <DatastoreLogsSkeleton />;
    case "databases.navMetrics":
      return <DatabaseMetricsSkeleton />;
    case "databases.navPlan":
      return <DatabasePlanSkeleton />;
    case "databases.navApps":
      return <DatabaseAppsSkeleton />;
    case "databases.navRecovery":
      return <DatabaseRecoverySkeleton database={database} />;
    default:
      return <DatabaseInfoSkeleton />;
  }
}

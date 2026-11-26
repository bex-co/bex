import { PageScroll } from "@/common/components/page-frame";
import {
  Outlet,
  createFileRoute,
  useNavigate,
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
  DatastoreLogsSkeleton,
} from "@/common/components/route-skeletons";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useScrollToHashedCard } from "@/common/hooks/use-scroll-to-hashed-card";
import {
  resourceLoadErrorVariant,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useDatabase } from "@/features/databases/hooks/use-database";
import type { DatabaseDetailView } from "@/features/databases/types";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { DatabaseRowActions } from "@/features/databases/components/database-row-actions";
import { databasePageLabelKey } from "@/features/databases/components/database-nav";
import { DatabaseDetailProvider } from "@/features/databases/lib/database-detail";
import { DatabaseDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
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
  useLoaderErrorRetry(Route.useLoaderData(), Route.useParams().databaseId);
  return <DatabaseDetailLayout />;
}

/**
 * A Postgres database's pages share this layout (w3/m641), as Render's do:
 * the header (name, status and actions), the open page's heading, which is
 * its sidebar entry's label, and an `<Outlet/>` for the page. The sidebar is
 * DashboardSidebar's DatabaseSidebar. The layout reads the database once and
 * hands it to its pages through DatabaseDetailProvider.
 */
function DatabaseDetailLayout({
  routePending = false,
}: {
  /**
   * Rendered as the route's pendingComponent while its title loader runs:
   * the same chrome with the page's skeleton, never the live `<Outlet/>`.
   */
  routePending?: boolean;
}) {
  const { databaseId } = Route.useParams();
  const { t } = useTranslations();
  const navigate = useNavigate();
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

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted database.
  useNotFoundRedirect(
    !routePending && resourceNotFound(loaded, loading, error),
  );
  const loadErrorVariant = routePending
    ? null
    : resourceLoadErrorVariant(loaded, loading, error);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const pageLabelKey = databasePageLabelKey(pathname, databaseId);
  useScrollToHashedCard(pathname, database !== null);

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
            <div className="flex min-w-0 max-w-full items-center gap-2">
              <h1 className="truncate text-xl font-semibold">
                {database.name}
              </h1>
              <DatabaseStatusBadge database={database} />
            </div>
          ) : (
            <div
              data-skeleton-region="resource-title"
              className="flex min-w-0 max-w-full items-center gap-2"
            >
              {/* The h1's text-xl line box is 28px (w4/m209). */}
              <Skeleton className="h-7 w-48" />
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
              {/* The ready actions trigger is an `icon-sm` button, 32px
                  (w3/082). */}
              <Skeleton className="size-8" />
            </div>
          )}
        </div>

        <PageScroll
          data-skeleton-region={!database ? "active-page" : undefined}
        >
          <div className="mx-auto w-full max-w-5xl space-y-6">
            {pageLabelKey ? (
              // Every page opens with its name, the sidebar's label for it.
              <h2 className="text-lg leading-7 font-semibold tracking-tight">
                {t(pageLabelKey)}
              </h2>
            ) : null}
            {loadErrorVariant ? (
              <ResourceLoadError
                onRetry={() => void refetch()}
                variant={loadErrorVariant}
              />
            ) : database ? (
              <DatabaseDetailProvider value={{ database, refetch, lifecycle }}>
                <Outlet />
              </DatabaseDetailProvider>
            ) : (
              <DatabasePageSkeleton labelKey={pageLabelKey} database={loaded} />
            )}
          </div>
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/**
 * The open page's own pending frame, the one its route shows on a page
 * switch, while the layout has no database yet. `database` is the cache's
 * copy when there is one (a workspace still being adopted).
 */
function DatabasePageSkeleton({
  labelKey,
  database,
}: {
  labelKey: ReturnType<typeof databasePageLabelKey>;
  database: DatabaseDetailView | null;
}) {
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

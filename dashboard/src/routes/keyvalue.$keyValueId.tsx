import { PageScroll } from "@/common/components/page-frame";
import {
  Outlet,
  createFileRoute,
  useRouterState,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  DatastoreLogsSkeleton,
  DatastoreMetricsSkeleton,
  KeyValueInfoSkeleton,
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
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import {
  KeyValueHeaderActions,
  KeyValuePlanBadge,
} from "@/features/keyvalue/components/key-value-header";
import { DatastoreHeaderActionsSkeleton } from "@/common/components/datastore-connect-menu";
import { keyValuePageLabelKey } from "@/features/keyvalue/components/key-value-nav";
import { KeyValueDetailProvider } from "@/features/keyvalue/lib/key-value-detail";
import { KEY_VALUE_HASH_ALIASES } from "@/features/keyvalue/lib/key-value-pages";
import { KeyValueDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/keyvalue/$keyValueId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms: the layout's chrome and the open page's
  // skeleton without the live <Outlet/>, so the title-loader wait shows the
  // real frame instead of the router-level blank that used to flash white,
  // and never a second live copy of the page (as the database layout does).
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
          query: KeyValueDocument,
          variables: { id: params.keyValueId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.keyValue?.name?.trim() ? data.keyValue : null),
    ),
  // Each page leads this with its own name (keyValuePageHead).
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (keyValue) => [
        keyValue.name,
        translatedText("keyvalue.resourceType"),
      ]),
      match,
    ),
});

function PendingRouteComponent() {
  return <KeyValueDetailLayout routePending />;
}

function RouteComponent() {
  // A roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useLoaderErrorRetry(Route.useLoaderData(), Route.useParams().keyValueId);
  return <KeyValueDetailLayout />;
}

/**
 * A Key Value instance's pages share this layout (w3/m649), as Render's do:
 * the header (name, status, plan badge, View docs and Connect, w3/m675), the
 * open page's heading, which is its sidebar entry's label, and an `<Outlet/>`
 * for the page. The sidebar is
 * DashboardSidebar's KeyValueSidebar. The layout reads the instance once and
 * hands it to its pages through KeyValueDetailProvider.
 */
function KeyValueDetailLayout({
  routePending = false,
}: {
  /**
   * Rendered as the route's pendingComponent while its title loader runs:
   * the same chrome with the page's skeleton, never the live `<Outlet/>`.
   */
  routePending?: boolean;
}) {
  const { keyValueId } = Route.useParams();
  const { t } = useTranslations();
  const { keyValue: loaded, loading, error, refetch } = useKeyValue(keyValueId);
  // Judge the pages' controls in the instance's own workspace (w4/641, as
  // w6/091 for services): until it is the selection, the skeleton stays up so
  // no role-gated control paints for the wrong workspace first.
  const adopting = useAdoptResourceWorkspace(
    keyValueId,
    routePending ? null : loaded?.ownerId,
  );
  const keyValue = adopting || routePending ? null : loaded;

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted instance.
  useNotFoundRedirect(
    !routePending && resourceNotFound(loaded, loading, error),
  );
  const loadErrorVariant = routePending
    ? null
    : resourceLoadErrorVariant(loaded, loading, error);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const pageLabelKey = keyValuePageLabelKey(pathname, keyValueId);
  useScrollToHashedCard(pathname, keyValue !== null, KEY_VALUE_HASH_ALIASES);

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!keyValue ? "keyvalue-detail" : undefined}
      >
        <div
          data-skeleton-region={!keyValue ? "resource-header" : undefined}
          className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6"
        >
          {keyValue ? (
            <div className="flex min-w-0 max-w-full items-center gap-2">
              <h1 className="truncate text-xl font-semibold">
                {keyValue.name}
              </h1>
              <KeyValueStatusBadge keyValue={keyValue} />
              <KeyValuePlanBadge keyValue={keyValue} />
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
          {keyValue ? (
            <KeyValueHeaderActions keyValue={keyValue} />
          ) : (
            <DatastoreHeaderActionsSkeleton />
          )}
        </div>

        <PageScroll
          data-skeleton-region={!keyValue ? "active-page" : undefined}
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
            ) : keyValue ? (
              <KeyValueDetailProvider value={{ keyValue, refetch }}>
                <Outlet />
              </KeyValueDetailProvider>
            ) : (
              <KeyValuePageSkeleton labelKey={pageLabelKey} />
            )}
          </div>
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/**
 * The open page's own pending frame, the one its route shows on a page
 * switch, while the layout has no instance yet.
 */
function KeyValuePageSkeleton({
  labelKey,
}: {
  labelKey: ReturnType<typeof keyValuePageLabelKey>;
}) {
  switch (labelKey) {
    case "keyvalue.navLogs":
      return <DatastoreLogsSkeleton />;
    case "keyvalue.navMetrics":
      return <DatastoreMetricsSkeleton kind="keyvalue" />;
    default:
      return <KeyValueInfoSkeleton />;
  }
}

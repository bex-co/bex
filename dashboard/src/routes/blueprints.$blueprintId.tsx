import { useState } from "react";
import { X } from "lucide-react";
import { PageScroll } from "@/common/components/page-frame";
import {
  Outlet,
  createFileRoute,
  useRouterState,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  resourceLoadErrorVariant,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  BlueprintHeaderSkeleton,
  BlueprintResourcesSkeleton,
  BlueprintSettingsSkeleton,
  BlueprintSyncPageSkeleton,
  BlueprintSyncsSkeleton,
} from "@/common/components/route-skeletons";
import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";
import { BlueprintResourceSuffix } from "@/features/blueprints/components/blueprint-resource-suffix";
import { BlueprintManualSync } from "@/features/blueprints/components/blueprint-manual-sync";
import {
  blueprintPageLabelKey,
  isBlueprintSyncPage,
} from "@/features/blueprints/components/blueprint-nav";
import { BlueprintDetailProvider } from "@/features/blueprints/lib/blueprint-detail";
import { canonicalBlueprintLinkId } from "@/features/blueprints/lib/link-id";
import { useBlueprint } from "@/features/blueprints/hooks/use-blueprint";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { BlueprintDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/blueprints/$blueprintId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms: the layout's chrome and the open page's
  // skeleton without the live <Outlet/>, so the title-loader wait shows the
  // real frame instead of the router-level blank that used to flash white,
  // and never a second live copy of the page (as the datastore layouts do).
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
          query: BlueprintDocument,
          variables: {
            id: canonicalBlueprintLinkId(params.blueprintId),
          },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.blueprint?.name?.trim() ? data.blueprint : null),
    ),
  // Each page leads this with its own name (blueprintPageHead).
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (blueprint) => [
        blueprint.name,
        translatedText("blueprints.resourceType"),
      ]),
      match,
    ),
});

function PendingRouteComponent() {
  return <BlueprintDetailLayout routePending />;
}

function RouteComponent() {
  // A roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useLoaderErrorRetry(Route.useLoaderData(), Route.useParams().blueprintId);
  return <BlueprintDetailLayout />;
}

/**
 * A Blueprint's pages share this layout (w3/m677), as Render's do: the header
 * (the name with bex's resource suffix and status, and Render's one action,
 * Manual sync), Render's dismissible Sync Error banner when Manual sync could
 * not plan (w3/m678), the open page's heading, which is its sidebar entry's
 * label (a sync's page and Settings head themselves), and an `<Outlet/>` for
 * Resources, Syncs, a sync or Settings. The sidebar is DashboardSidebar's BlueprintSidebar. The
 * layout reads the Blueprint once and hands it to its pages through
 * BlueprintDetailProvider.
 */
function BlueprintDetailLayout({
  routePending = false,
}: {
  /**
   * Rendered as the route's pendingComponent while its title loader runs:
   * the same chrome with the page's skeleton, never the live `<Outlet/>`.
   */
  routePending?: boolean;
}) {
  const { blueprintId: linkId } = Route.useParams();
  // A bookmarked /blueprints/blp-… still opens: the pages address the
  // Blueprint by the exs- id the API returns (w2/m269).
  const blueprintId = canonicalBlueprintLinkId(linkId);
  const { t } = useTranslations();
  const {
    blueprint: loaded,
    loading,
    error,
    refetch,
  } = useBlueprint(blueprintId);
  // Judge the pages' controls in the Blueprint's own workspace (w4/641, as
  // w6/091 for services): until it is the selection, the skeleton stays up so
  // no Manual sync or Disconnect paints for the wrong workspace first.
  const adopting = useAdoptResourceWorkspace(
    blueprintId,
    routePending ? null : loaded?.ownerId,
  );
  const blueprint = adopting || routePending ? null : loaded;

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted Blueprint.
  useNotFoundRedirect(
    !routePending && resourceNotFound(loaded, loading, error),
  );
  const loadErrorVariant = routePending
    ? null
    : resourceLoadErrorVariant(loaded, loading, error);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const pageLabelKey = blueprintPageLabelKey(pathname, linkId);
  const syncPage = isBlueprintSyncPage(pathname, linkId);
  // A sync's page heads itself with the sync, and Settings with Render's
  // Settings card (w3/m679).
  const headsItself = syncPage || pageLabelKey === "blueprints.navSettings";
  // Manual sync's refusal (the server's reason, mutationErrorMessage), kept
  // with the Blueprint it is about: another Blueprint's pages reuse this
  // layout without remounting it.
  const [planRefusal, setPlanRefusal] = useState<{
    blueprintId: string;
    reason: string;
  } | null>(null);

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!blueprint ? "blueprint-detail" : undefined}
      >
        {blueprint ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
            <div className="flex min-w-0 max-w-full items-center gap-2">
              <h1 className="truncate text-xl font-semibold">
                {blueprint.name}
              </h1>
              <BlueprintResourceSuffix suffix={blueprint.resourceSuffix} />
              <BlueprintStatusBadge status={blueprint.status} />
            </div>
            {/* BlueprintHeaderSkeleton reserves this container's box. */}
            <div className="flex shrink-0 items-center gap-2">
              <BlueprintManualSync
                blueprintId={blueprint.id}
                linkId={linkId}
                onError={(reason) =>
                  setPlanRefusal(
                    reason ? { blueprintId: blueprint.id, reason } : null,
                  )
                }
              />
            </div>
          </div>
        ) : (
          <BlueprintHeaderSkeleton />
        )}
        {blueprint && planRefusal?.blueprintId === blueprint.id ? (
          <div className="px-4 pt-4 sm:px-6">
            <Alert
              variant="destructive"
              className="mx-auto max-w-5xl pr-12"
              data-testid="blueprint-sync-error"
            >
              <AlertTitle>{t("blueprints.syncErrorBannerTitle")}</AlertTitle>
              <AlertDescription>{planRefusal.reason}</AlertDescription>
              <Button
                variant="ghost"
                size="icon"
                className="absolute top-2 right-2 size-8"
                aria-label={t("blueprints.syncErrorBannerDismiss")}
                onClick={() => setPlanRefusal(null)}
              >
                <X aria-hidden="true" />
              </Button>
            </Alert>
          </div>
        ) : null}

        <PageScroll
          data-skeleton-region={!blueprint ? "active-page" : undefined}
        >
          <div className="mx-auto w-full max-w-5xl space-y-6">
            {pageLabelKey && !headsItself ? (
              // Every page opens with its name, the sidebar's label for it.
              <h2 className="text-lg leading-7 font-semibold tracking-tight">
                {t(pageLabelKey)}
              </h2>
            ) : null}
            {loadErrorVariant ? (
              <ResourceLoadError onRetry={refetch} variant={loadErrorVariant} />
            ) : blueprint ? (
              <BlueprintDetailProvider value={{ blueprint, linkId }}>
                <Outlet />
              </BlueprintDetailProvider>
            ) : (
              <BlueprintPageSkeleton
                labelKey={pageLabelKey}
                syncPage={syncPage}
              />
            )}
          </div>
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/**
 * The open page's own pending frame, the one its route shows on a page
 * switch, while the layout has no Blueprint yet.
 */
function BlueprintPageSkeleton({
  labelKey,
  syncPage,
}: {
  labelKey: ReturnType<typeof blueprintPageLabelKey>;
  syncPage: boolean;
}) {
  if (syncPage) return <BlueprintSyncPageSkeleton />;
  switch (labelKey) {
    case "blueprints.navSyncs":
      return <BlueprintSyncsSkeleton />;
    case "blueprints.navSettings":
      return <BlueprintSettingsSkeleton />;
    default:
      return <BlueprintResourcesSkeleton />;
  }
}

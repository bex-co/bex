import { PageScroll } from "@/common/components/page-frame";
import { useState, type ReactNode } from "react";
import { Outlet, useRouterState } from "@tanstack/react-router";
import { LogIn, TriangleAlert } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  resourceLoadErrorVariant,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useSignInAgain } from "@/common/lib/auth/sign-in-again";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { serviceTabLabelKey } from "@/features/services/components/service-nav";
import {
  useCachedServiceSource,
  useCachedServiceType,
} from "@/features/services/hooks/use-cached-service-type";
import { useLatestDeploy } from "@/features/deploys/hooks/use-latest-deploy";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import {
  ServiceBaseProvider,
  type ServiceBase,
} from "@/features/services/lib/service-base";
import { ServiceDeletingProvider } from "@/features/services/lib/service-deleting";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
} from "@/features/services/components/service-detail-header";
import { ServiceRouteContentSkeleton } from "@/common/components/route-skeletons";
import { DeployHistorySkeleton } from "@/features/deploys/components/deploy-history-skeleton";

/**
 * Shared chrome for every per-service page (Render's service-detail shape): the
 * overview header with lifecycle actions, the resource-scoped nav, and an
 * `<Outlet/>` for the active tab. Rendered by both service-detail route trees —
 * `/services/$serviceId` (compute types) and `/static/$serviceId` (static_site,
 * Render parity w5/m57). The parent route's network-only loader fills Apollo's
 * normalized cache for the title; this layout's cache-first `useServer` read
 * consumes that same result for the header without a second title-only request.
 */
export function ServiceDetailLayout({
  serviceId,
  base = "/services",
  routePending = false,
}: {
  serviceId: string;
  base?: ServiceBase;
  /**
   * Rendered as the parent route's pendingComponent while its title loader
   * runs. The pending frame shows the same chrome with the tab skeleton and
   * must never mount the live `<Outlet/>`: reusing the full layout as its own
   * pendingComponent streamed a second, unadopted copy of the tab page into
   * the SSR shell, which hydration then left behind as a dead duplicate
   * (w4/m236; same shape as w3/m44).
   */
  routePending?: boolean;
}) {
  const { service, loading, error, refetch, ownerId } = useServer(serviceId);
  // Judge this page's permissions in the service's own workspace (w6/091).
  useAdoptResourceWorkspace(serviceId, routePending ? null : ownerId);
  // A Billing member views no deploys (w9/m621): the header's latest-deploy
  // badge is left out and its read never fires.
  const activityDenied = useCapabilities().denied("can_view_activity");
  const { deploy: latestDeploy, loading: latestDeployLoading } =
    useLatestDeploy(serviceId, { skip: activityDenied });
  const { pending, run } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();
  const signInAgain = useSignInAgain();
  const [deletingSelf, setDeletingSelf] = useState(false);
  // The open tab's own name, from the pathname alone, so the loading frame
  // and the loaded tab carry the same heading.
  const tabLabelKey = useRouterState({
    select: (s) => serviceTabLabelKey(s.location.pathname, serviceId, base),
  });

  // Unknown service id: redirect home with a toast (w9/m55) — covering every
  // child tab at once. A genuine query error is excluded; it keeps the inline
  // retry state below, and so is this service's own delete (w4/340).
  useNotFoundRedirect(
    !routePending && !deletingSelf && resourceNotFound(service, loading, error),
  );

  // `base` (the route tree this layout renders under) rides context so the
  // sidebar + every tab's intra-detail link stays within /services or /static
  // (Render parity, w5/m57) without re-sniffing the router.
  let content: ReactNode;
  // A failed `server(id)` query is not evidence that the service is absent.
  // Keep it distinct from not-found so schema skew, auth failures, and backend
  // outages never masquerade as a deleted service. And keep an EXPIRED session
  // distinct from a backend outage (w3/m80 t002): a 401 gets the "your session
  // has expired — sign in" card (the auth link is usually already redirecting),
  // never the "The request to bex-api failed" retry card the screenshot showed.
  const loadErrorVariant = resourceLoadErrorVariant(service, loading, error);
  const expired = loadErrorVariant === "unauthenticated";
  // The pending frame never decides error/not-found and never shows the live
  // service: it is chrome + the active tab's skeleton until the real mount
  // replaces it.
  const shownService = routePending ? null : service;
  if (!routePending && loadErrorVariant) {
    content = (
      <DashboardLayout>
        <PageScroll>
          <div className="mx-auto w-full max-w-5xl">
            <Card>
              <CardContent className="flex flex-col items-center justify-center gap-4 py-16 text-center">
                {expired ? (
                  <LogIn className="text-muted-foreground h-8 w-8" />
                ) : (
                  <TriangleAlert className="text-destructive h-8 w-8" />
                )}
                <div>
                  <p className="mb-1 font-medium">
                    {t(
                      expired
                        ? "common.sessionExpiredTitle"
                        : "services.detailErrorTitle",
                    )}
                  </p>
                  <p className="text-muted-foreground text-sm">
                    {t(
                      expired
                        ? "common.sessionExpiredBody"
                        : "services.detailErrorBody",
                    )}
                  </p>
                </div>
                {expired ? (
                  <Button size="sm" onClick={signInAgain}>
                    {t("common.signIn")}
                  </Button>
                ) : (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void refetch()}
                  >
                    {t("common.tryAgain")}
                  </Button>
                )}
              </CardContent>
            </Card>
          </div>
        </PageScroll>
      </DashboardLayout>
    );
  } else if (!routePending && !service && !loading) {
    // Unknown service id: the redirect above is already in flight. Render only
    // skeleton chrome for the WHOLE detail (every tab) — never let a child tab
    // borrow another service's data (the 2026-07-09 phantom-service bug the
    // fixed stub also guards against), so the `<Outlet/>` stays unmounted.
    content = (
      <DashboardLayout>
        <div
          className="relative min-h-0 flex-1 overflow-auto"
          data-route-skeleton={
            base === "/static" ? "static-active-tab" : "service-active-tab"
          }
        >
          <ServiceDetailHeaderSkeleton />
        </div>
      </DashboardLayout>
    );
  } else {
    content = (
      <DashboardLayout>
        {/* `relative` makes the scroller the containing block of every
            absolutely positioned descendant (a badge's screen-reader text, a
            hit-area overlay), so it clips them. Without it they were placed
            against the document and stretched it past the shell: on a phone
            the window itself scrolled 400px into a blank band. */}
        <div
          data-page-scroll=""
          data-touch-region=""
          className="relative min-h-0 flex-1 overflow-auto"
        >
          {shownService ? (
            <ServiceDetailHeader
              service={shownService}
              latestDeploy={latestDeploy}
              latestDeployLoading={latestDeployLoading && !latestDeploy}
              pending={pending?.id === shownService.id ? pending.action : null}
              onRun={run}
            />
          ) : (
            <PendingServiceHeader serviceId={serviceId} />
          )}
          {/* The page container the section layout's queries measure. The
              scroller has no top padding (the service header scrolls away
              above), so a sticky bar needs no offset. */}
          <div className="@container/page p-4 sm:p-6">
            {/* One column for every tab, the same measure as the header band
                above it: the tab's content starts under the service name and
                ends under Manual Deploy at every width, so switching tabs
                never moves the page's edges. Past that measure (1920) the
                band and the column center together rather than leaving the
                spare width all on the right. A centered narrower column
                stranded each tab off the header's axis (296px right of the
                name at 1920). */}
            <div
              data-skeleton-region={!shownService ? "active-tab" : undefined}
              className="mx-auto w-full max-w-7xl space-y-6"
            >
              {tabLabelKey ? (
                // Every tab opens with its name — the sidebar's label for it
                // — between the service's h1 and the tab's own cards.
                <h2 className="text-lg leading-7 font-semibold tracking-tight">
                  {t(tabLabelKey)}
                </h2>
              ) : null}
              {shownService ? (
                <Outlet />
              ) : (
                <PendingTabContent serviceId={serviceId} base={base} />
              )}
            </div>
          </div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <ServiceBaseProvider value={base}>
      <ServiceDeletingProvider value={setDeletingSelf}>
        {content}
      </ServiceDeletingProvider>
    </ServiceBaseProvider>
  );
}

/**
 * The header's frame, with the repository and registry-credential items the
 * cache says the ready header will draw, so its wrapping row ends on the same
 * line (w3/094).
 */
function PendingServiceHeader({ serviceId }: { serviceId: string }) {
  return (
    <ServiceDetailHeaderSkeleton source={useCachedServiceSource(serviceId)} />
  );
}

/**
 * Before the server read lands, the list cache may already know the type. The
 * deploy history draws its route's own skeleton, which shows the
 * auto-deploy-off banner from the cached service as the ready page does
 * (w3/064): a tab switch paints this frame first, then that skeleton.
 */
function PendingTabContent({
  serviceId,
  base,
}: {
  serviceId: string;
  base: ServiceBase;
}) {
  return (
    <ServiceRouteContentSkeleton
      base={base}
      serviceType={useCachedServiceType(serviceId)}
      deploys={<DeployHistorySkeleton />}
    />
  );
}

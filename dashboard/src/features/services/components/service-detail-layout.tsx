import { useState, type ReactNode } from "react";
import { Outlet, useRouterState } from "@tanstack/react-router";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  ResourceNotFound,
  ResourceStatePage,
  ResourceStateView,
} from "@/common/components/resource-not-found";
import { useResourceNotFoundLinks } from "@/common/hooks/use-resource-state-links";
import { useMissingServiceContext } from "@/common/hooks/use-missing-resource-context";
import { resourceReadState } from "@/common/lib/resource-read-state";
import type { RouteResource } from "@/common/lib/document-head";
import { useTranslations } from "@/common/hooks/use-translations";
import { isCronType } from "@/features/services/lib/service-type";
import { useServer } from "@/features/services/hooks/use-server";
import {
  serviceHeaderHidden,
  serviceTabLabelKey,
} from "@/features/services/components/service-nav";
import {
  useCachedRunsInstances,
  useCachedServiceName,
  useCachedServiceShellAccess,
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
 * overview header with lifecycle actions (left out on Logs, Metrics, Shell and
 * a deploy's page, as Render does), the resource-scoped nav, and an
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
  routeState,
}: {
  serviceId: string;
  base?: ServiceBase;
  /** The route loader's verdict on the service, which alone can tell a
   *  refused read from a missing one (`resourceReadState`). */
  routeState?: RouteResource<unknown>["state"];
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
  const [deletingSelf, setDeletingSelf] = useState(false);
  // The open tab's own name, from the pathname alone, so the loading frame
  // and the loaded tab carry the same heading.
  const tabLabelKey = useRouterState({
    select: (s) => serviceTabLabelKey(s.location.pathname, serviceId, base),
  });
  // From the pathname, so the pending frame leaves the header's frame out too.
  const headerHidden = useRouterState({
    select: (s) => serviceHeaderHidden(s.location.pathname, serviceId, base),
  });

  // `base` (the route tree this layout renders under) rides context so the
  // sidebar + every tab's intra-detail link stays within /services or /static
  // (Render parity, w5/m57) without re-sniffing the router.
  let content: ReactNode;
  // A dead id keeps its URL and draws Render's "Service not found" (or
  // "Service deleted", "Cron job …") in place of every tab at once; a refused
  // read draws Access denied, an expired session Sign in, any other failure
  // "An error occurred" (w3/m706). This service's own delete is not a dead id:
  // its flow owns the toast and the landing (w4/340), so the frame stays up.
  // The pending frame never decides a state and never shows the live service:
  // it is chrome + the active tab's skeleton until the real mount replaces it.
  const readState = routePending
    ? null
    : resourceReadState(service, loading, error, routeState);
  const state = readState === "not-found" && deletingSelf ? null : readState;
  const shownService = routePending ? null : service;
  if (state) {
    content = (
      <ResourceStatePage>
        <ResourceStateView
          state={state}
          id={serviceId}
          notFound={<ServiceNotFound serviceId={serviceId} />}
        />
      </ResourceStatePage>
    );
  } else if (!routePending && !service && !loading) {
    // This service's own delete reads its id as gone while its flow lands
    // elsewhere. Render only skeleton chrome for the WHOLE detail (every tab)
    // meanwhile — never let a child tab borrow another service's data (the
    // 2026-07-09 phantom-service bug the fixed stub also guards against), so
    // the `<Outlet/>` stays unmounted.
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
          {headerHidden ? null : shownService ? (
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
              scroller has no top padding (the service header, where there is
              one, scrolls away above), so a sticky bar needs no offset. */}
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
 * Render's missing service: "Service not found", or "Service deleted" with
 * its name when the cache still holds it ("Cron job …" for a cron job), Go to
 * project when it sat in one, and Return home.
 */
function ServiceNotFound({ serviceId }: { serviceId: string }) {
  const { name, projectId, type } = useMissingServiceContext(serviceId);
  const links = useResourceNotFoundLinks(projectId);
  return (
    <ResourceNotFound
      kind={type && isCronType(type) ? "cronJob" : "service"}
      id={serviceId}
      name={name}
      links={links}
    />
  );
}

/**
 * The header's frame, with the repository and registry-credential items the
 * cache says the ready header will draw, so its wrapping row ends on the same
 * line (w3/094), and the cached name, so its first row does too (w3/m555).
 */
function PendingServiceHeader({ serviceId }: { serviceId: string }) {
  return (
    <ServiceDetailHeaderSkeleton
      source={useCachedServiceSource(serviceId)}
      name={useCachedServiceName(serviceId)}
    />
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
      runsInstances={useCachedRunsInstances(serviceId)}
      shellAccess={useCachedServiceShellAccess(serviceId)}
      deploys={<DeployHistorySkeleton />}
    />
  );
}

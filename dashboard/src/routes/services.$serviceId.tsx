import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  deriveServiceType,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import { ServiceDetailLayout } from "@/features/services/components/service-detail-layout";
import { loadServiceDetail } from "@/features/services/lib/service-detail-loader";
import {
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/services/$serviceId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms: the layout's chrome + tab skeleton
  // without the live <Outlet/>, so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white — and never a
  // second live copy of the tab page (w4/m236; w3/m44's shape).
  pendingComponent: PendingRouteComponent,
  pendingMs: 0,
  // No-arg requireAuth: `next` is the requested href (the old form passed the
  // literal "$serviceId" pattern), so a login bounce returns to the actual
  // service URL — id- or name-shaped.
  beforeLoad: requireAuth(),
  // Both /services/$serviceId and /static/$serviceId parent loaders call
  // loadServiceDetail, which canonicalizes the base (static_site → /static,
  // everything else → /services) via redirectPreservingSuffix — so an old
  // /services/<static-id>/<subpath> bookmark still lands, just under /static
  // with the subpath intact. Bare-URL aliases still settle in the index route
  // (service-root-redirect.ts). Breadcrumb chrome is fire-and-forget warmed
  // inside loadServiceDetail (does not gate the title loader).
  loader: ({ context, params, location, cause }) =>
    loadServiceDetail(
      context.client,
      params.serviceId,
      "/services",
      location,
      cause,
      context.workspaceId,
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (service) => [
        service.displayName?.trim() || service.name,
        translatedText(
          SERVICE_TYPE_LABEL[deriveServiceType(service.type ?? "")],
        ),
      ]),
      match,
    ),
});

function PendingRouteComponent() {
  const { serviceId } = Route.useParams();
  return (
    <ServiceDetailLayout serviceId={serviceId} base="/services" routePending />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  // A roll-window loader failure dehydrates `state: "error"` — re-run it once
  // (w1/m52) so the title/head recover with the data; the layout's Apollo
  // reads below heal on their own.
  useLoaderErrorRetry(Route.useLoaderData(), serviceId);
  return <ServiceDetailLayout serviceId={serviceId} base="/services" />;
}

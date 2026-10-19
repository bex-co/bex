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

/**
 * The canonical detail tree for a static_site (Render parity, w5/m57 — Render
 * serves static sites at `dashboard.render.com/static/<id>`). It renders the
 * same shared ServiceDetailLayout + tab pages as `/services/$serviceId`;
 * `loadServiceDetail` canonicalizes the base, so a non-static service reaching
 * `/static/<id>` is bounced to `/services/<id>` before render (loop-free — the
 * `/services` tree renders every type).
 */
export const Route = createFileRoute("/static/$serviceId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // A dedicated pending frame at 0ms — same rationale as
  // /services/$serviceId: skeleton chrome without the live <Outlet/>
  // (w4/m236).
  pendingComponent: PendingRouteComponent,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, location, cause }) =>
    loadServiceDetail(
      context.client,
      params.serviceId,
      "/static",
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
    <ServiceDetailLayout serviceId={serviceId} base="/static" routePending />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  useLoaderErrorRetry(Route.useLoaderData(), serviceId);
  return <ServiceDetailLayout serviceId={serviceId} base="/static" />;
}

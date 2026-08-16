import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  deriveServiceType,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import { ServiceDetailLayout } from "@/features/services/components/service-detail-layout";
import { ServerDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/services/$serviceId")({
  staticData: { chrome: true },
  component: RouteComponent,
  // The page doubles as its own pending state at 0ms: ServiceDetailLayout
  // renders full chrome + a header skeleton while its Apollo read loads
  // (tolerating the absent loaderData), so the title-loader wait shows the
  // real frame instead of the router-level blank that used to flash white.
  pendingComponent: RouteComponent,
  pendingMs: 0,
  // No-arg requireAuth: `next` is the requested href (the old form passed the
  // literal "$serviceId" pattern), so a login bounce returns to the actual
  // service URL — id- or name-shaped.
  beforeLoad: requireAuth(),
  // A static_site is canonical under /static; this /services layout keeps the
  // plain loader (no reverse-redirect) so a static service reached at an old
  // /services/<id>/<subpath> bookmark still renders — canonicalization of the
  // bare service URL happens in the index route (service-root-redirect.ts).
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: ServerDocument,
          variables: { id: params.serviceId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) =>
        data?.server &&
        (data.server.displayName?.trim() || data.server.name?.trim())
          ? data.server
          : null,
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

function RouteComponent() {
  const { serviceId } = Route.useParams();
  // A roll-window loader failure dehydrates `state: "error"` — re-run it once
  // (w1/m52) so the title/head recover with the data; the layout's Apollo
  // reads below heal on their own.
  useLoaderErrorRetry(Route.useLoaderData(), serviceId);
  return <ServiceDetailLayout serviceId={serviceId} base="/services" />;
}

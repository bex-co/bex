import { createFileRoute } from "@tanstack/react-router";
import { redirectServiceRootByType } from "@/features/services/lib/service-root-redirect";

/**
 * The service root has no page of its own — Render's service URL lands on the
 * type's primary tab (Deploys for compute types; Events for a static_site,
 * which has no Deploys tab — Render parity, w5/m57), under its canonical base
 * (`/services` or `/static`). A static service reached here at `/services/<id>`
 * is sent straight to `/static/<id>/events`. Every identity fact the retired
 * Overview tab carried now lives in the detail header; keeping the route as a
 * redirect lets existing deep links (services list, project page, bookmarks)
 * still resolve.
 */
export const Route = createFileRoute("/services/$serviceId/")({
  beforeLoad: ({ context, params }) =>
    redirectServiceRootByType(context.client, params.serviceId),
});

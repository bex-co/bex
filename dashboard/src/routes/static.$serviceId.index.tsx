import { createFileRoute } from "@tanstack/react-router";
import { redirectServiceRootByType } from "@/features/services/lib/service-root-redirect";

/**
 * Bare `/static/<id>` lands on the static_site's primary tab, Events (Render
 * parity, w5/m57). Shares the resolver with the `/services` index so the two
 * bare URLs stay in lockstep; a non-static service reaching here is redirected
 * out to `/services/<id>/deploys`.
 */
export const Route = createFileRoute("/static/$serviceId/")({
  beforeLoad: ({ context, params }) =>
    redirectServiceRootByType(context.client, params.serviceId),
});

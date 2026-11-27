import { createFileRoute } from "@tanstack/react-router";
import { redirectServiceSubpath } from "@/features/services/lib/service-root-redirect";

/**
 * Every sub-path of a service with no page of its own (w3/m707): Render's
 * `/disks`, `/metrics/<period>`, `/shell/<instance>` and a deploy's own
 * sub-paths arrive here through the `/web/<id>/…` aliases and land on bex's
 * page, query and hash kept (serviceSubpathLanding). Anything else lands on
 * the service's primary tab, as Render's `*` route sends it to the root.
 */
export const Route = createFileRoute("/services/$serviceId/$")({
  beforeLoad: ({ context, params, location }) =>
    redirectServiceSubpath(
      context.client,
      params.serviceId,
      params._splat,
      "/services",
      location,
    ),
});

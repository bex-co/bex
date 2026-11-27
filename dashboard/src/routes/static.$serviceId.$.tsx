import { createFileRoute } from "@tanstack/react-router";
import { redirectServiceSubpath } from "@/features/services/lib/service-root-redirect";

/**
 * Every sub-path of `/static/<id>` with no page of its own (w3/m707): Render
 * serves every service type's sub-paths under its static segment too, so
 * `/static/<id>/metrics/<period>` opens Metrics on its window, and a page a
 * static site has not (Logs, Shell, Disk, Scaling) lands on its Events, as
 * Render's `*` route sends it to the root. Another type's service lands on
 * the page under `/services`.
 */
export const Route = createFileRoute("/static/$serviceId/$")({
  beforeLoad: ({ context, params, location }) =>
    redirectServiceSubpath(
      context.client,
      params.serviceId,
      params._splat,
      "/static",
      location,
    ),
});

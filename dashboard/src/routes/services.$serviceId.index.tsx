import { createFileRoute } from "@tanstack/react-router";
import { redirectServiceRoot } from "@/features/services/lib/service-root-redirect";

/**
 * The service root has no page of its own — Render's service URL lands on
 * Deploys, and every identity fact the retired Overview tab carried now lives in
 * the detail header. Keep the route as a redirect so existing deep links (the
 * services list, the project page, bookmarks) still resolve.
 */
export const Route = createFileRoute("/services/$serviceId/")({
  beforeLoad: ({ params }) => {
    redirectServiceRoot(params.serviceId);
  },
});

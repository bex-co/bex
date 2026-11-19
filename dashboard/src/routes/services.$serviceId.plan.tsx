import { createFileRoute } from "@tanstack/react-router";
import { redirectToComputePage } from "@/features/services/lib/compute-page";

/**
 * The service's Plan tab (w5/m7) became the Compute page's Compute section,
 * as on Render since 2026-08-26 (w6/m483). Bookmarks and links minted before
 * land there, query and hash kept.
 */
export const Route = createFileRoute("/services/$serviceId/plan")({
  beforeLoad: ({ params, location }) => {
    redirectToComputePage(params.serviceId, location);
  },
});

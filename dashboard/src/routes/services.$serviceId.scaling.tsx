import { createFileRoute } from "@tanstack/react-router";
import {
  MANUAL_SCALING_ANCHOR,
  redirectToComputePage,
} from "@/features/services/lib/compute-page";

/**
 * The service's Scaling tab (w7/m43) became the Compute page's scaling
 * sections, as on Render since 2026-08-26: "open your service's Compute page
 * and scroll down to the Manual Scaling section" (render.com/docs/scaling,
 * w6/m483). Bookmarks and links minted before land on that section.
 */
export const Route = createFileRoute("/services/$serviceId/scaling")({
  beforeLoad: ({ params, location }) => {
    redirectToComputePage(params.serviceId, location, MANUAL_SCALING_ANCHOR);
  },
});

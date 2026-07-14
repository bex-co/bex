import { createFileRoute, redirect } from "@tanstack/react-router";

/**
 * The service root has no page of its own — Render's service URL lands on
 * Events, and every identity fact the retired Overview tab carried now lives in
 * the detail header. Keep the route as a redirect so existing deep links (the
 * services list, the project page, bookmarks) still resolve.
 */
export const Route = createFileRoute("/services/$serviceId/")({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/services/$serviceId/events",
      params: { serviceId: params.serviceId },
      replace: true,
    });
  },
});

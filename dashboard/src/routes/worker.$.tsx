import { createFileRoute } from "@tanstack/react-router";
import { redirectRenderAlias } from "@/common/lib/render-alias";

/** Render-shaped alias (docs/render-artifacts/dashboard-routes.md) — redirects to the canonical route. */
export const Route = createFileRoute("/worker/$")({
  beforeLoad: ({ params, location }) => {
    redirectRenderAlias("worker", params._splat, location);
  },
});

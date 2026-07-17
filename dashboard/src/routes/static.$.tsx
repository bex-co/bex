import { createFileRoute } from "@tanstack/react-router";
import { redirectRenderAlias } from "@/common/lib/render-alias";

/** Render-shaped alias (docs/render-artifacts/dashboard-routes.md) — redirects to the canonical route. */
export const Route = createFileRoute("/static/$")({
  beforeLoad: ({ params, location }) => {
    redirectRenderAlias("static", params._splat, location);
  },
});

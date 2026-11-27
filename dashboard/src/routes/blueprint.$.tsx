import { createFileRoute, redirect } from "@tanstack/react-router";
import { redirectRenderAlias, splatParts } from "@/common/lib/render-alias";

/**
 * Render's Blueprint URLs, `/blueprint/<id>[/<page>]` (w3/m677,
 * docs/render-artifacts/blueprint-pages.md): redirects to bex's
 * `/blueprints/<id>[/<page>]` with the query and hash kept. `/blueprint/new`
 * and `/blueprint/generate` are routes of their own: the create page, and the
 * Create Blueprint page Generate Blueprint opens (w3/m681). A path under
 * `generate` names no Blueprint and goes to `/`, as the page does with
 * nothing selected.
 */
export const Route = createFileRoute("/blueprint/$")({
  beforeLoad: ({ params, location }) => {
    if (splatParts(params._splat)[0] === "generate") {
      throw redirect({ to: "/", replace: true });
    }
    redirectRenderAlias("blueprint", params._splat, location);
  },
});

import { createFileRoute, redirect } from "@tanstack/react-router";
import { redirectRenderAlias, splatParts } from "@/common/lib/render-alias";

/**
 * Render's Blueprint URLs, `/blueprint/<id>[/<page>]` (w3/m677,
 * docs/render-artifacts/blueprint-pages.md): redirects to bex's
 * `/blueprints/<id>[/<page>]` with the query and hash kept. `/blueprint/new`
 * is its own route, the create page. Render's `/blueprint/generate` with
 * nothing selected goes to `/`, and so does bex's until w3/m681 builds the
 * page.
 */
export const Route = createFileRoute("/blueprint/$")({
  beforeLoad: ({ params, location }) => {
    if (splatParts(params._splat)[0] === "generate") {
      throw redirect({ to: "/", replace: true });
    }
    redirectRenderAlias("blueprint", params._splat, location);
  },
});

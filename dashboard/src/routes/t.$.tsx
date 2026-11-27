import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's older team scheme, `/t/<tea-id>/settings[/…]` (w3/m705; Render
 * routes it beside `/w/<tea-id>/settings/*` to the same page): opens what
 * `/w/<tea-id>/…` opens, whose alias selects the workspace after checking the
 * caller is in it.
 */
export const Route = createFileRoute("/t/$")({
  beforeLoad: ({ params, location }) => {
    redirectPreservingSuffix(`/w/${params._splat ?? ""}`, location);
  },
});

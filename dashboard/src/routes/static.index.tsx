import { createFileRoute } from "@tanstack/react-router";
import { redirectRenderAlias } from "@/common/lib/render-alias";

/**
 * Bare `/static` has no Render meaning — send it to the workspace overview
 * (redirectRenderAlias's empty-splat behavior). The static-site detail tree
 * lives under `/static/$serviceId`; `/static/new` is the create URL.
 */
export const Route = createFileRoute("/static/")({
  staticData: { chrome: true },
  beforeLoad: ({ location }) => {
    redirectRenderAlias("static", undefined, location);
  },
});

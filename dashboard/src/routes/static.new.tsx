import { createFileRoute } from "@tanstack/react-router";
import { redirectRenderAlias } from "@/common/lib/render-alias";

/**
 * `/static/new` is the static-site create URL (Render's New-menu shape). It is
 * NOT a service id, so it must not fall into `/static/$serviceId` — this more
 * specific route redirects to the shared create wizard preselected to
 * static_site (RENDER_CREATE_LANDINGS.static, w5/m47/m57).
 */
export const Route = createFileRoute("/static/new")({
  beforeLoad: ({ location }) => {
    redirectRenderAlias("static", "new", location);
  },
});

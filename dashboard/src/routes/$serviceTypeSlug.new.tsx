import { createFileRoute } from "@tanstack/react-router";
import {
  redirectPreservingSuffix,
  renderCreateSlugLanding,
} from "@/common/lib/render-alias";

/**
 * Render's `/:serviceTypeSlug/new` (w3/m705): `/WEB/new`, `/postgres/new`,
 * `/kv/new`, `/background-worker/new` and the rest of Render's create slugs
 * open their create page, and any other name the `/new` landing, with the
 * query and hash kept. The five service types' own `/<type>/new` pages, and
 * every other route under a first segment bex serves, answer first.
 */
export const Route = createFileRoute("/$serviceTypeSlug/new")({
  beforeLoad: ({ params, location }) => {
    redirectPreservingSuffix(
      renderCreateSlugLanding(params.serviceTypeSlug),
      location,
    );
  },
});

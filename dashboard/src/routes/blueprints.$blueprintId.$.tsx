import { createFileRoute, redirect } from "@tanstack/react-router";
import { blueprintSubpathLanding } from "@/features/blueprints/lib/blueprint-pages";
import { blueprintPageRoute } from "@/features/blueprints/components/blueprint-nav";

/**
 * Every sub-path of a Blueprint with no page of its own (w3/m677): Render's
 * `/sync` and `/sync/<syncId>` arrive here through the `/blueprint/<id>/…`
 * alias (and bex-api's sync links once w3/m678 points them there) and land on
 * Syncs; anything else lands on Resources, as Render's `*` route shows
 * Resources. The query and hash are kept.
 */
export const Route = createFileRoute("/blueprints/$blueprintId/$")({
  beforeLoad: ({ params, location }) => {
    const page = blueprintSubpathLanding(params._splat);
    throw redirect({
      to: page ? blueprintPageRoute(page) : "/blueprints/$blueprintId",
      params: { blueprintId: params.blueprintId },
      search: true,
      hash: location.hash || undefined,
      replace: true,
    });
  },
});

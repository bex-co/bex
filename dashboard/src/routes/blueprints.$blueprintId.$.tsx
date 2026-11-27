import { createFileRoute, redirect } from "@tanstack/react-router";
import { blueprintSubpathLanding } from "@/features/blueprints/lib/blueprint-pages";
import { blueprintPageRoute } from "@/features/blueprints/components/blueprint-nav";

/**
 * Every sub-path of a Blueprint with no page of its own (w3/m677): one under
 * `sync` past a sync's page lands on Syncs, anything else on Resources, as
 * Render's `*` route shows Resources. A sync's page, `/sync[/<syncId>]`, is
 * its own route (w3/m678). The query and hash are kept.
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

import { createFileRoute, redirect } from "@tanstack/react-router";
import { databaseSubpathLanding } from "@/features/databases/lib/database-pages";
import { databasePageRoute } from "@/features/databases/components/database-nav";

/**
 * Every sub-path of a Postgres database with no page of its own (w3/m641):
 * Render's `/upgrade`, `/high-availability-upgrade`,
 * `/metrics/<period>` and `/sharing` arrive here through the `/d/<id>/…`
 * alias, and land where their control is, query and hash kept
 * (databaseSubpathLanding). Anything else lands on Info, as Render's `*`
 * route shows Info.
 */
export const Route = createFileRoute("/databases/$databaseId/$")({
  beforeLoad: ({ params, location }) => {
    const landing = databaseSubpathLanding(params._splat);
    throw redirect({
      to: landing.page
        ? databasePageRoute(landing.page)
        : "/databases/$databaseId",
      params: { databaseId: params.databaseId },
      search: (prev) => ({ ...prev, ...landing.search }),
      hash: location.hash || landing.hash,
      replace: true,
    });
  },
});

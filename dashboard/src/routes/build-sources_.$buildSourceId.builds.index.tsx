import { createFileRoute, redirect } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's build source Builds URL, `/build-sources/<bds-id>/builds`, and a
 * build's, `…/builds?build=<bld-id>` (Render's `urls` chunk, read 2026-11-27,
 * w3/m705): bex lists the builds on the build source's index and gives a
 * build a page of its own, `…/builds/<bld-id>`.
 */
export const Route = createFileRoute("/build-sources_/$buildSourceId/builds/")({
  validateSearch: (search: Record<string, unknown>): { build?: string } => ({
    build:
      typeof search.build === "string" && search.build !== ""
        ? search.build
        : undefined,
  }),
  beforeLoad: ({ params, search, location }) => {
    if (search.build) {
      throw redirect({
        to: "/build-sources/$buildSourceId/builds/$buildId",
        params: { buildSourceId: params.buildSourceId, buildId: search.build },
        replace: true,
      });
    }
    redirectPreservingSuffix(
      `/build-sources/${params.buildSourceId}`,
      location,
    );
  },
});

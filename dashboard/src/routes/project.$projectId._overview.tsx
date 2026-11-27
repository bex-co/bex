import { createFileRoute, redirect } from "@tanstack/react-router";
import { ProjectOverviewPageSkeleton } from "@/common/components/route-skeletons";
import { parseLegacyProjectSearch } from "@/features/projects/lib/resource-filter";
import { ProjectPage } from "@/features/projects/pages/project-page";

/**
 * A project's page and Render's `/project/<id>/environment/<envId>` (w1/m682):
 * one pathless layout over both, so the address going back to the project's
 * own URL after the user scrolls neither remounts the page nor moves it.
 * Each child only names its path; this layout renders the page.
 *
 * Links from before w1/m682 carried the selected environment, search and
 * type in the query (`?env=`, `?q=`, `?kind=`). Each section now keeps its
 * own, so such a link redirects once: `?env=<id>` to that environment's URL
 * (`unassigned` names the project-only section), anything else to the
 * project's own.
 */
export const Route = createFileRoute("/project/$projectId/_overview")({
  component: ProjectPage,
  pendingComponent: ProjectOverviewPageSkeleton,
  validateSearch: parseLegacyProjectSearch,
  beforeLoad: ({ search, params }) => {
    if (search.env) {
      throw redirect({
        to: "/project/$projectId/environment/$environmentId",
        params: { projectId: params.projectId, environmentId: search.env },
        search: {},
        replace: true,
      });
    }
    if (search.q || search.kind) {
      throw redirect({
        to: "/project/$projectId",
        params: { projectId: params.projectId },
        search: {},
        replace: true,
      });
    }
  },
});

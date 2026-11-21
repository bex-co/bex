import { createFileRoute, redirect } from "@tanstack/react-router";
import { ProjectMetricsViewPageSkeleton } from "@/common/components/route-skeletons";
import {
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";
import { defaultSavedSearchId } from "@/features/logs/lib/project-log-views";
import {
  parseProjectMetricsSearch,
  type ProjectLogSearch,
} from "@/features/logs/lib/project-log-search";
import { ProjectMetricsViewPage } from "@/features/metrics/components/project-metrics-view-page";
import { SavedSearchType } from "@/graphql/definitions";

/**
 * A project's Service Metrics view (w3/m508), at Render's own address:
 * `/project/{prj}/observability/metrics?savedSearchId=svs-…&r=12h`. A URL
 * without a view opens the project's first Service Metrics view, as the log
 * views' routes do.
 */
export const Route = createFileRoute(
  "/project/$projectId/observability/metrics",
)({
  component: RouteComponent,
  pendingComponent: ProjectMetricsViewPageSkeleton,
  validateSearch: parseProjectMetricsSearch,
  beforeLoad: async ({ context, params, search, cause }) => {
    if (search.savedSearchId) return;
    const first = await defaultSavedSearchId(
      context.client,
      params.projectId,
      SavedSearchType.Metrics,
      cause,
    );
    if (first) {
      throw redirect({
        to: "/project/$projectId/observability/metrics",
        params,
        search: { ...search, savedSearchId: first },
        replace: true,
      });
    }
  },
  // The page and the sidebar read the project's views from the cache this
  // warms, so the page mounts with its view (its pending state until then).
  loader: async ({ context, params, cause, parentMatchPromise }) => {
    await defaultSavedSearchId(
      context.client,
      params.projectId,
      SavedSearchType.Metrics,
      cause,
    );
    return (await parentMatchPromise).loaderData;
  },
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (project) => [
        `${project.name} / ${translatedText("metrics.serviceMetricsTitle")}`,
      ]),
      match,
    ),
});

function RouteComponent() {
  const { projectId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const write = (next: ProjectLogSearch) => {
    if (
      next.savedSearchId === search.savedSearchId &&
      next.r === search.r &&
      next.rangeStart === search.rangeStart &&
      next.rangeEnd === search.rangeEnd
    ) {
      return;
    }
    // Functional form: its result is committed as is, so a cleared custom
    // bound really leaves the URL (w7/m42).
    void navigate({ search: () => next, replace: true });
  };
  return (
    <ProjectMetricsViewPage
      projectId={projectId}
      search={search}
      onSearchChange={write}
    />
  );
}

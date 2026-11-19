import { createFileRoute, redirect } from "@tanstack/react-router";
import { ProjectLogViewPageSkeleton } from "@/common/components/route-skeletons";
import {
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";
import { ProjectLogViewPage } from "@/features/logs/components/project-log-view-page";
import { defaultSavedSearchId } from "@/features/logs/lib/project-log-views";
import {
  parseProjectLogSearch,
  type ProjectLogSearch,
} from "@/features/logs/lib/project-log-search";
import { SavedSearchType } from "@/graphql/definitions";

/**
 * A project's Request Logs view (w3/m505), at Render's own address:
 * `/project/{prj}/observability/logs/request?savedSearchId=svs-…&r=1h`. A URL
 * without a view opens the project's first Request Logs view, as
 * Render's sidebar links always name one.
 */
export const Route = createFileRoute(
  "/project/$projectId/observability/logs/request",
)({
  component: RouteComponent,
  pendingComponent: ProjectLogViewPageSkeleton,
  validateSearch: parseProjectLogSearch,
  beforeLoad: async ({ context, params, search, cause }) => {
    if (search.savedSearchId) return;
    const first = await defaultSavedSearchId(
      context.client,
      params.projectId,
      SavedSearchType.RequestLogs,
      cause,
    );
    if (first) {
      throw redirect({
        to: "/project/$projectId/observability/logs/request",
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
      SavedSearchType.RequestLogs,
      cause,
    );
    return (await parentMatchPromise).loaderData;
  },
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (project) => [
        `${project.name} / ${translatedText("logs.requestLogsTitle")}`,
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
    <ProjectLogViewPage
      projectId={projectId}
      viewType={SavedSearchType.RequestLogs}
      search={search}
      onSearchChange={write}
    />
  );
}

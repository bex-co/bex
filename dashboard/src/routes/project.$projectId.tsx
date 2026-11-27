import { createFileRoute, Outlet } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ProjectRouteSkeleton } from "@/common/components/route-skeletons";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  ResourceStatePage,
  ResourceStateView,
} from "@/common/components/resource-not-found";
import { ownDeleteActive } from "@/common/lib/own-delete";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { ProjectDocument } from "@/graphql/definitions";
import { mapProject } from "@/features/projects/hooks/use-projects";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";

/**
 * Chrome-preserving pending state at 0ms (the sidebar-navigation white-flash
 * fix). Unlike the other detail routes this can't reuse `RouteComponent` as
 * its own pending: that component dereferences the loader's result (absent
 * while pending) and renders the child `<Outlet/>`, whose pages read the
 * parent's loaderData. So keep the layout — DashboardSidebar swaps in the
 * contextual ProjectSidebar off the route params — and put the default
 * spinner where the child page will land.
 */
export const Route = createFileRoute("/project/$projectId")({
  staticData: { chrome: true },
  component: RouteComponent,
  pendingComponent: ProjectRouteSkeleton,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: ProjectDocument,
          variables: { id: params.projectId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.project?.name?.trim() ? mapProject(data.project) : null),
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (project) => [project.name]),
      match,
    ),
});

/**
 * Shared chrome for every per-project page (Overview, Settings): just the
 * dashboard sidebar/header — DashboardSidebar swaps in the contextual
 * ProjectSidebar for any `/project/$projectId*` route, so no in-page tab nav
 * is needed here. Each child route (`.index.tsx`, `.settings.tsx`) owns its
 * own header content and content padding.
 */
function RouteComponent() {
  // A dead project id keeps its URL and draws Render's Page not found for
  // every child page; a refused read draws Access denied, an expired session
  // Sign in, and any other failure "An error occurred" (w3/m706). A failed
  // loader re-runs once (w1/m52), so a roll-window failure self-heals instead
  // of stranding the error until a manual reload.
  const { projectId } = Route.useParams();
  const projectResult = Route.useLoaderData();
  useLoaderErrorRetry(projectResult, projectId);
  // Judge every child page in the project's own workspace (w4/641, as w6/091
  // for services): until it is the selection, the children stay on their
  // pending skeleton so no role-gated control paints for the wrong workspace.
  const adopting = useAdoptResourceWorkspace(
    projectId,
    projectResult.state === "ready" ? projectResult.resource.ownerId : null,
  );
  // The dashboard's own delete reads the project as gone while its flow
  // lands home with its toast (w4/645): the frame stays up meanwhile.
  const state =
    projectResult.state === "ready" ||
    (projectResult.state === "not-found" && ownDeleteActive())
      ? null
      : projectResult.state;
  if (state) {
    return (
      <ResourceStatePage>
        <ResourceStateView state={state} id={projectId} />
      </ResourceStatePage>
    );
  }
  // Every project page is a touch region, as a service's pages are (w3/099):
  // below the desktop layout or on a coarse pointer its buttons, selects and
  // inputs take the 44px target (style.css). `contents` keeps the child's
  // scroller a flex item of the shell's column.
  return (
    <DashboardLayout>
      <div data-touch-region="" className="contents">
        {adopting || projectResult.state !== "ready" ? (
          <ProjectRouteSkeleton />
        ) : (
          <Outlet />
        )}
      </div>
    </DashboardLayout>
  );
}

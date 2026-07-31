import { createFileRoute, Outlet } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import RoutePending from "@/common/root-route/route-pending";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { ProjectDocument } from "@/features/projects/api/operations";

/**
 * Chrome-preserving pending state at 0ms (the sidebar-navigation white-flash
 * fix). Unlike the other detail routes this can't reuse `RouteComponent` as
 * its own pending: that component dereferences the loader's result (absent
 * while pending) and renders the child `<Outlet/>`, whose pages read the
 * parent's loaderData. So keep the layout — DashboardSidebar swaps in the
 * contextual ProjectSidebar off the route params — and put the default
 * spinner where the child page will land.
 */
function ProjectPending() {
  return (
    <DashboardLayout>
      <RoutePending />
    </DashboardLayout>
  );
}

export const Route = createFileRoute("/project/$projectId")({
  component: RouteComponent,
  pendingComponent: ProjectPending,
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
      (data) => (data?.project?.name?.trim() ? data.project : null),
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
  // A dead project id redirects home for every child page (w9/m55); a query
  // error passes through so the children keep their inline error states —
  // and re-runs the loader once (w1/m52), so a roll-window failure self-heals
  // instead of stranding "Couldn't load resources" until a manual reload.
  const { projectId } = Route.useParams();
  const projectResult = Route.useLoaderData();
  useNotFoundRedirect(projectResult.state === "not-found");
  useLoaderErrorRetry(projectResult, projectId);
  return (
    <DashboardLayout>
      <Outlet />
    </DashboardLayout>
  );
}

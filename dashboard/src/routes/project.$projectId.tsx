import { createFileRoute, Outlet } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
} from "@/common/lib/document-head";
import { ProjectDocument } from "@/features/projects/api/operations";

export const Route = createFileRoute("/project/$projectId")({
  component: RouteComponent,
  beforeLoad: requireAuth(),
  loader: ({ context, params }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: ProjectDocument,
          variables: { id: params.projectId },
          fetchPolicy: "network-only",
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
  // error passes through so the children keep their inline error states.
  const projectResult = Route.useLoaderData();
  useNotFoundRedirect(projectResult.state === "not-found");
  return (
    <DashboardLayout>
      <Outlet />
    </DashboardLayout>
  );
}

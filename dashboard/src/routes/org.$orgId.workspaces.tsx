import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { loadOrganizationRoute } from "@/features/organizations/lib/organization-route-loader";
import { OrganizationWorkspacesRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { OrganizationWorkspacesPage } from "@/features/organizations/pages/organization-workspaces-page";

export const Route = createFileRoute("/org/$orgId/workspaces")({
  staticData: { chrome: true },
  component: OrganizationWorkspacesRoute,
  pendingComponent: OrganizationWorkspacesRouteSkeleton,
  beforeLoad: requireAuth(),
  loader: (match) => loadOrganizationRoute(match, "workspaces"),
  head: ({ match }) =>
    translatedTitleHead("organizations.workspacesTitle", match),
});

function OrganizationWorkspacesRoute() {
  const { orgId } = Route.useParams();
  return <OrganizationWorkspacesPage organizationId={orgId} />;
}

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
  // `?new=workspace` opens an Owner's New Workspace dialog: the URL owns it,
  // so Render's `/organization/<id>/new/workspace` lands in it (w3/m705).
  validateSearch: (search: Record<string, unknown>): { new?: "workspace" } => ({
    new: search.new === "workspace" ? "workspace" : undefined,
  }),
  head: ({ match }) =>
    translatedTitleHead("organizations.workspacesTitle", match),
});

function OrganizationWorkspacesRoute() {
  const { orgId } = Route.useParams();
  const { new: newParam } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <OrganizationWorkspacesPage
      organizationId={orgId}
      createOpen={newParam === "workspace"}
      onCreateOpenChange={(open) =>
        void navigate({
          search: open ? { new: "workspace" } : {},
          replace: true,
        })
      }
    />
  );
}

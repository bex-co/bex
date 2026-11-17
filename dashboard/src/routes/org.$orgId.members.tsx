import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { loadOrganizationRoute } from "@/features/organizations/lib/organization-route-loader";
import { OrganizationMembersRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { OrganizationMembersPage } from "@/features/organizations/pages/organization-members-page";

export const Route = createFileRoute("/org/$orgId/members")({
  staticData: { chrome: true },
  component: OrganizationMembersRoute,
  pendingComponent: OrganizationMembersRouteSkeleton,
  beforeLoad: requireAuth(),
  loader: (match) => loadOrganizationRoute(match, "members"),
  head: ({ match }) => translatedTitleHead("organizations.membersTitle", match),
});

function OrganizationMembersRoute() {
  const { orgId } = Route.useParams();
  return <OrganizationMembersPage organizationId={orgId} />;
}

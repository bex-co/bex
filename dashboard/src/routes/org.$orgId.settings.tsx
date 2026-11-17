import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { loadOrganizationRoute } from "@/features/organizations/lib/organization-route-loader";
import { OrganizationSettingsRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { OrganizationSettingsPage } from "@/features/organizations/pages/organization-settings-page";

export const Route = createFileRoute("/org/$orgId/settings")({
  staticData: { chrome: true },
  component: OrganizationSettingsRoute,
  pendingComponent: OrganizationSettingsRouteSkeleton,
  beforeLoad: requireAuth(),
  loader: (match) => loadOrganizationRoute(match, "settings"),
  head: ({ match }) =>
    translatedTitleHead("organizations.settingsTitle", match),
});

function OrganizationSettingsRoute() {
  const { orgId } = Route.useParams();
  return <OrganizationSettingsPage organizationId={orgId} />;
}

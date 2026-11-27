import { OrganizationPageShell } from "@/features/organizations/components/organization-page-shell";
import { OrganizationWorkspacesCard } from "@/features/organizations/components/organization-workspaces-card";
import { OrganizationWorkspacesCardSkeleton } from "@/features/organizations/components/organization-route-skeletons";

/**
 * `/org/$orgId/workspaces` (w7/m464): the page Render reaches through "the
 * workspace dropdown at the top of the left pane", listing the org's
 * workspaces with their privacy and the Primary label.
 */
export function OrganizationWorkspacesPage({
  organizationId,
  createOpen,
  onCreateOpenChange,
}: {
  organizationId: string;
  /** The New Workspace dialog's open state, when the URL owns it. */
  createOpen?: boolean;
  onCreateOpenChange?: (open: boolean) => void;
}) {
  return (
    <OrganizationPageShell
      organizationId={organizationId}
      section="workspaces"
      skeleton={
        <OrganizationWorkspacesCardSkeleton organizationId={organizationId} />
      }
    >
      {({ organization, role }) => (
        <OrganizationWorkspacesCard
          organization={organization}
          role={role}
          createOpen={createOpen}
          onCreateOpenChange={onCreateOpenChange}
        />
      )}
    </OrganizationPageShell>
  );
}

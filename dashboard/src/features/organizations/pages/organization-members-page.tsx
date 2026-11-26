import { OrganizationPageShell } from "@/features/organizations/components/organization-page-shell";
import { OrganizationMembersCard } from "@/features/organizations/components/organization-members-card";
import { OrganizationMembersCardSkeleton } from "@/features/organizations/components/organization-route-skeletons";

/**
 * `/org/$orgId/members` (w7/m464): the org's people and their org roles, the
 * Owner's invites and member controls. A Guest is told the list is for
 * Owners and Members.
 */
export function OrganizationMembersPage({
  organizationId,
}: {
  organizationId: string;
}) {
  return (
    <OrganizationPageShell
      organizationId={organizationId}
      section="members"
      skeleton={
        <OrganizationMembersCardSkeleton organizationId={organizationId} />
      }
    >
      {({ organization, role, workspaceName }) => (
        <OrganizationMembersCard
          organizationId={organizationId}
          role={role}
          workspaceName={workspaceName}
          workspace={(id) => organization.workspaces.find((w) => w.id === id)}
        />
      )}
    </OrganizationPageShell>
  );
}

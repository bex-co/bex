import { OrganizationPageShell } from "@/features/organizations/components/organization-page-shell";
import { OrganizationNameCard } from "@/features/organizations/components/organization-name-card";
import { OrganizationAuditLogsCard } from "@/features/organizations/components/organization-audit-logs-card";
import { OrganizationSettingsSkeleton } from "@/features/organizations/components/organization-route-skeletons";

/**
 * `/org/$orgId/settings` (w7/m464): the org's name, and Render's Audit Logs
 * section, which only Owners see ("Only org members with the Org Owner role
 * can export these audit logs"), as a workspace's is hidden from non-admins.
 */
export function OrganizationSettingsPage({
  organizationId,
}: {
  organizationId: string;
}) {
  return (
    <OrganizationPageShell
      organizationId={organizationId}
      section="settings"
      skeleton={
        <OrganizationSettingsSkeleton organizationId={organizationId} />
      }
    >
      {({ organization, role }) => (
        <>
          <OrganizationNameCard
            organizationId={organizationId}
            name={organization.name}
            canRename={role === "OWNER"}
          />
          {role === "OWNER" ? (
            <OrganizationAuditLogsCard organizationId={organizationId} />
          ) : null}
        </>
      )}
    </OrganizationPageShell>
  );
}

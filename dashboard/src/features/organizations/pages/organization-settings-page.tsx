import { OrganizationPageShell } from "@/features/organizations/components/organization-page-shell";
import { OrganizationNameCard } from "@/features/organizations/components/organization-name-card";
import { OrganizationSecurityCard } from "@/features/organizations/components/organization-security-card";
import { OrganizationAuditLogsCard } from "@/features/organizations/components/organization-audit-logs-card";
import { OrganizationSettingsSkeleton } from "@/features/organizations/components/organization-route-skeletons";

/**
 * `/org/$orgId/settings` (w7/m464): the org's name; Security (w7/m466), the
 * org's login requirements, which everyone in the org reads and Owners
 * change; and Render's Audit Logs section, where only Owners export the log
 * ("Only org members with the Org Owner role can export these audit logs")
 * and manage its audit-log API keys (w1/m657), and everyone else reads
 * Render's no-permission texts.
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
          <OrganizationSecurityCard
            organization={organization}
            canChange={role === "OWNER"}
          />
          <OrganizationAuditLogsCard
            organizationId={organizationId}
            isOwner={role === "OWNER"}
          />
        </>
      )}
    </OrganizationPageShell>
  );
}

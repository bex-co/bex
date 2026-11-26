import { Card, CardContent } from "@/common/components/ui/card";
import { AuditLogExport } from "@/features/audit/components/audit-log-export";
import { OrganizationAuditLogKeys } from "@/features/organizations/components/organization-audit-log-keys";
import {
  OrganizationAuditLogsCardHeader,
  OrganizationAuditNoPermission,
} from "@/features/organizations/components/organization-settings-frame";

/** Render's `list-organization-audit-logs`, as bex-api serves it (ADR115 D7). */
function organizationAuditLogsPath(organizationId: string): string {
  return `/v1/organizations/${encodeURIComponent(organizationId)}/audit-logs`;
}

/**
 * Org Settings → Audit Logs, Render's section. An Owner exports the org's
 * log (Render: "Only org members with the Org Owner role can export these
 * audit logs … Select a start and end date, then click Export as CSV"; the
 * same export as a workspace's, at most 90 days, paging the org's REST list,
 * so the file has the same columns) and manages the org's audit-log API keys
 * (w1/m657). Everyone else sees the section as Render shows it to them, with
 * its no-permission texts in place of the export and the keys.
 */
export function OrganizationAuditLogsCard({
  organizationId,
  isOwner,
}: {
  organizationId: string;
  isOwner: boolean;
}) {
  return (
    <Card>
      <OrganizationAuditLogsCardHeader />
      <CardContent className="space-y-6">
        {isOwner ? (
          <AuditLogExport
            subjectId={organizationId}
            path={organizationAuditLogsPath(organizationId)}
          />
        ) : (
          <OrganizationAuditNoPermission />
        )}
        <OrganizationAuditLogKeys
          organizationId={organizationId}
          canManage={isOwner}
        />
      </CardContent>
    </Card>
  );
}

import { Card, CardContent } from "@/common/components/ui/card";
import { AuditLogExport } from "@/features/audit/components/audit-log-export";
import { OrganizationAuditLogsCardHeader } from "@/features/organizations/components/organization-settings-frame";

/** Render's `list-organization-audit-logs`, as bex-api serves it (ADR115 D7). */
function organizationAuditLogsPath(organizationId: string): string {
  return `/v1/organizations/${encodeURIComponent(organizationId)}/audit-logs`;
}

/**
 * Org Settings → Audit Logs (Render: "Only org members with the Org Owner role
 * can export these audit logs … Select a start and end date, then click
 * Export as CSV"). The same export as a workspace's, at most 90 days, paging
 * the org's REST list, so the file has the same columns. The page renders it
 * for Owners only.
 */
export function OrganizationAuditLogsCard({
  organizationId,
}: {
  organizationId: string;
}) {
  return (
    <Card>
      <OrganizationAuditLogsCardHeader />
      <CardContent>
        <AuditLogExport
          subjectId={organizationId}
          path={organizationAuditLogsPath(organizationId)}
        />
      </CardContent>
    </Card>
  );
}

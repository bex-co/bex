import { useTranslations } from "@/common/hooks/use-translations";
import { useAuditLog } from "@/features/audit/hooks/use-audit-log";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import { SettingsSection } from "@/features/auth/pages/settings-page/settings-section";
import { ConnectedAgentsPanel } from "@/features/connected-agents/components/connected-agents-panel";
import { ActiveSessionsPanel } from "@/features/sessions/components/active-sessions-panel";

/**
 * Settings → Security & Compliance (w4/m15): the grouping that houses the
 * workspace's audit trail plus the account's own security controls —
 * Connected Agents (w4/m18, the revocation surface m16's remembered consent
 * shipped without) and Active Sessions (w4/006, folded into m18) — mirroring
 * Render's placement of audit-log export under Workspace Settings →
 * Compliance.
 *
 * Unlike the Audit Log (admin-only, `useAuditLog`'s `forbidden` gate), the two
 * account-security cards are visible to every member, so the section itself
 * always renders — only `AuditLogPanel` is conditionally hidden inside it, so
 * a non-admin still sees the section but not the audit trail it can't read.
 *
 * The workspace comes from `useAuditLog` itself, which reads the switcher's
 * selection (w6/m14 t006) — this section used to pass `useCurrentWorkspace()`'s
 * `workspaces[0]`, which pinned the table to the account's original workspace.
 */
export function SecurityComplianceSection() {
  const { t } = useTranslations();
  const audit = useAuditLog();

  return (
    <SettingsSection
      title={t("auth.securityComplianceSection")}
      description={t("auth.securityComplianceSectionSubtitle")}
    >
      <ConnectedAgentsPanel />
      <ActiveSessionsPanel />
      {audit.forbidden ? null : <AuditLogPanel state={audit} />}
    </SettingsSection>
  );
}

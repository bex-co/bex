import { useTranslations } from "@/common/hooks/use-translations";
import { useAuditLog } from "@/features/audit/hooks/use-audit-log";
import { AuditLogPanel } from "@/features/audit/components/audit-log-panel";
import { SettingsSection } from "@/features/auth/pages/settings-page/settings-section";

/**
 * Settings → Security & Compliance (w4/m15): the grouping that houses the
 * workspace's audit trail, mirroring Render's placement of audit-log export
 * under Workspace Settings → Compliance. Owns the single `useAuditLog` query so
 * it can gate the whole section on `forbidden` — a non-admin (who can't read
 * the audit log) sees no lonely section heading, preserving the rule that a
 * non-admin shouldn't learn the audit feature exists.
 *
 * This is the intended home for the planned session-management (w4/006) and MFA
 * (m11) cards: those are visible to all members, so once either lands the
 * section renders unconditionally and this audit-only gate becomes moot.
 *
 * The workspace comes from `useAuditLog` itself, which reads the switcher's
 * selection (w6/m14 t006) — this section used to pass `useCurrentWorkspace()`'s
 * `workspaces[0]`, which pinned the table to the account's original workspace.
 */
export function SecurityComplianceSection() {
  const { t } = useTranslations();
  const audit = useAuditLog();

  // Only the admin-gated Audit Log lives here today; hide the whole section
  // (heading included) when it's forbidden so a non-admin sees nothing.
  if (audit.forbidden) return null;

  return (
    <SettingsSection
      title={t("auth.securityComplianceSection")}
      description={t("auth.securityComplianceSectionSubtitle")}
    >
      <AuditLogPanel state={audit} />
    </SettingsSection>
  );
}

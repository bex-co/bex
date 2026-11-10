import { useTranslations } from "@/common/hooks/use-translations";
import { SettingsSection } from "@/features/auth/pages/settings-page/settings-section";
import { ConnectedAgentsPanel } from "@/features/connected-agents/components/connected-agents-panel";
import { ActiveSessionsPanel } from "@/features/sessions/components/active-sessions-panel";

/**
 * Settings → Security & Compliance (w4/m15): the account's own security
 * controls — Connected Agents (w4/m18, the revocation surface m16's remembered
 * consent shipped without) and Active Sessions (w4/006, folded into m18). The
 * workspace audit trail moved to Workspace Settings → Audit Logs, Render's
 * place for it (w3/m305).
 */
export function SecurityComplianceSection() {
  const { t } = useTranslations();

  return (
    <SettingsSection
      id="security"
      title={t("auth.securityComplianceSection")}
      description={t("auth.securityComplianceSectionSubtitle")}
    >
      <ConnectedAgentsPanel />
      <ActiveSessionsPanel />
    </SettingsSection>
  );
}

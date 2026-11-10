import {
  Bell,
  Bot,
  KeyRound,
  ScrollText,
  Settings2,
  TriangleAlert,
  Users,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the workspace settings page — same right-rail
 * quick nav as the service settings page.
 */
export function WorkspaceSettingsNavigation({
  className,
  showDangerZone,
  showAuditLogs = true,
}: {
  className?: string;
  showDangerZone: boolean;
  /** Audit Logs is admin-only; a non-admin's page has no such section. */
  showAuditLogs?: boolean;
}) {
  const { t } = useTranslations();
  const items = [
    {
      href: "#general",
      label: t("workspaces.generalTitle"),
      icon: Settings2,
    },
    { href: "#team", label: t("team.title"), icon: Users },
    {
      href: "#notifications",
      label: t("notifications.workspaceTitle"),
      icon: Bell,
    },
    { href: "#agents", label: t("agentEnvironment.title"), icon: Bot },
    {
      href: "#registry-credentials",
      label: t("registryCredentials.title"),
      icon: KeyRound,
    },
    ...(showAuditLogs
      ? [{ href: "#audit-logs", label: t("audit.title"), icon: ScrollText }]
      : []),
    ...(showDangerZone
      ? [
          {
            href: "#danger-zone",
            label: t("workspaces.dangerZoneTitle"),
            icon: TriangleAlert,
          },
        ]
      : []),
  ];

  return (
    <SectionNavigation
      ariaLabel={t("workspaces.settingsNavigation")}
      items={items}
      className={className}
    />
  );
}

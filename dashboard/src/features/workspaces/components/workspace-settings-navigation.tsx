import { Settings2, TriangleAlert, Users } from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the workspace settings page — same right-rail
 * quick nav as the service settings page.
 */
export function WorkspaceSettingsNavigation({
  className,
}: {
  className?: string;
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
      href: "#danger-zone",
      label: t("workspaces.dangerZoneTitle"),
      icon: TriangleAlert,
    },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("workspaces.settingsNavigation")}
      items={items}
      className={className}
    />
  );
}

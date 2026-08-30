import { Blocks, KeyRound, ShieldCheck, Trash2, UserRound } from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

export function SettingsNavigation({ className }: { className?: string }) {
  const { t } = useTranslations();
  const items = [
    {
      href: "#account",
      label: t("auth.accountSection"),
      icon: UserRound,
    },
    {
      href: "#integrations",
      label: t("auth.integrationsSection"),
      icon: Blocks,
    },
    {
      href: "#access",
      label: t("auth.accessSection"),
      icon: KeyRound,
    },
    {
      href: "#security",
      label: t("auth.securityComplianceSection"),
      icon: ShieldCheck,
    },
    {
      href: "#danger-zone",
      label: t("auth.dangerZoneSection"),
      icon: Trash2,
    },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("auth.settingsNavigation")}
      items={items}
      className={className}
    />
  );
}

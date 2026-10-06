import {
  Bell,
  CirclePause,
  CirclePlay,
  FileText,
  Globe2,
  Hammer,
  HeartPulse,
  KeyRound,
  Network,
  Plug,
  Rocket,
  Settings2,
  TriangleAlert,
  Webhook,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  navigableSettingsSections,
  type NavigableSettingsSectionId,
  type ServiceSettingsSectionId,
} from "@/features/services/lib/settings-sections";
import type { en } from "@/i18n";

interface SectionItem {
  labelKey: keyof typeof en;
  icon: LucideIcon;
}

const SECTION_ITEMS: Record<NavigableSettingsSectionId, SectionItem> = {
  general: {
    labelKey: "services.generalTitle",
    icon: Settings2,
  },
  deploy: {
    labelKey: "services.deployTitle",
    icon: Rocket,
  },
  build: {
    labelKey: "services.buildTitle",
    icon: Hammer,
  },
  source: {
    labelKey: "services.sourceTitle",
    icon: Hammer,
  },
  "static-site": {
    labelKey: "services.staticTitle",
    icon: FileText,
  },
  domains: {
    labelKey: "services.domainsTitle",
    icon: Globe2,
  },
  networking: {
    labelKey: "services.networkingTitle",
    icon: Network,
  },
  "registry-credential": {
    labelKey: "services.registryCredentialSettingsTitle",
    icon: KeyRound,
  },
  notifications: {
    labelKey: "services.settingsNotificationsTitle",
    icon: Bell,
  },
  port: {
    labelKey: "services.settingsPortTitle",
    icon: Plug,
  },
  "health-checks": {
    labelKey: "services.settingsHealthChecksTitle",
    icon: HeartPulse,
  },
  maintenance: {
    labelKey: "services.maintenanceModeTitle",
    icon: Wrench,
  },
  "deploy-hook": {
    labelKey: "services.deployHookTitle",
    icon: Webhook,
  },
  suspend: {
    labelKey: "services.suspendCardTitle",
    icon: CirclePause,
  },
  "danger-zone": {
    labelKey: "services.dangerZoneTitle",
    icon: TriangleAlert,
  },
};

// A suspended service's card offers Resume, under the Suspend anchor.
const RESUME_ITEM: SectionItem = {
  labelKey: "services.resumeCardTitle",
  icon: CirclePlay,
};

export function ServiceSettingsNavigation({
  sections,
  suspended,
  className,
}: {
  sections: ServiceSettingsSectionId[];
  suspended: boolean;
  className?: string;
}) {
  const { t } = useTranslations();
  const items = navigableSettingsSections(sections).map((section) => {
    const item =
      section === "suspend" && suspended ? RESUME_ITEM : SECTION_ITEMS[section];
    return { href: `#${section}`, label: t(item.labelKey), icon: item.icon };
  });

  return (
    <SectionNavigation
      ariaLabel={t("services.settingsNavigation")}
      items={items}
      className={className}
    />
  );
}

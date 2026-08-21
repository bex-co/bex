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
  Rocket,
  Settings2,
  TriangleAlert,
  Webhook,
  Wrench,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

export type ServiceSettingsSection =
  | "general"
  | "deploy"
  | "build"
  | "source"
  | "static-site"
  | "domains"
  | "networking"
  | "registry-credential"
  | "notifications"
  | "health-checks"
  | "maintenance"
  | "deploy-hook"
  | "suspend"
  | "resume"
  | "danger-zone";

const SECTION_ITEMS = {
  general: {
    href: "#general",
    labelKey: "services.generalTitle",
    icon: Settings2,
  },
  deploy: {
    href: "#deploy",
    labelKey: "services.deployTitle",
    icon: Rocket,
  },
  build: {
    href: "#build",
    labelKey: "services.buildTitle",
    icon: Hammer,
  },
  source: {
    href: "#source",
    labelKey: "services.sourceTitle",
    icon: Hammer,
  },
  "static-site": {
    href: "#static-site",
    labelKey: "services.staticTitle",
    icon: FileText,
  },
  domains: {
    href: "#domains",
    labelKey: "services.domainsTitle",
    icon: Globe2,
  },
  networking: {
    href: "#networking",
    labelKey: "services.networkingTitle",
    icon: Network,
  },
  "registry-credential": {
    href: "#registry-credential",
    labelKey: "services.registryCredentialSettingsTitle",
    icon: KeyRound,
  },
  notifications: {
    href: "#notifications",
    labelKey: "services.settingsNotificationsTitle",
    icon: Bell,
  },
  "health-checks": {
    href: "#health-checks",
    labelKey: "services.settingsHealthChecksTitle",
    icon: HeartPulse,
  },
  maintenance: {
    href: "#maintenance",
    labelKey: "services.maintenanceModeTitle",
    icon: Wrench,
  },
  "deploy-hook": {
    href: "#deploy-hook",
    labelKey: "services.deployHookTitle",
    icon: Webhook,
  },
  suspend: {
    href: "#suspend",
    labelKey: "services.suspendCardTitle",
    icon: CirclePause,
  },
  resume: {
    href: "#suspend",
    labelKey: "services.resumeCardTitle",
    icon: CirclePlay,
  },
  "danger-zone": {
    href: "#danger-zone",
    labelKey: "services.dangerZoneTitle",
    icon: TriangleAlert,
  },
} as const;

export function ServiceSettingsNavigation({
  sections,
  className,
}: {
  sections: ServiceSettingsSection[];
  className?: string;
}) {
  const { t } = useTranslations();
  const items = sections.map((section) => {
    const { href, labelKey, icon } = SECTION_ITEMS[section];
    return { href, label: t(labelKey), icon };
  });

  return (
    <SectionNavigation
      ariaLabel={t("services.settingsNavigation")}
      items={items}
      className={className}
    />
  );
}

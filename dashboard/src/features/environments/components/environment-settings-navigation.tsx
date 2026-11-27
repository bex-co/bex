import {
  Layers,
  LockKeyhole,
  Network,
  Pencil,
  ShieldOff,
  Trash2,
  type LucideIcon,
} from "lucide-react";
import {
  SectionNavigation,
  type SectionNavigationItem,
} from "@/common/components/section-navigation";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import type { EnvironmentSettingsSection } from "@/features/environments/lib/settings";

const SECTION_COPY: Record<
  EnvironmentSettingsSection,
  { labelKey: Parameters<Translate>[0]; icon: LucideIcon }
> = {
  name: { labelKey: "environments.nameCardTitle", icon: Pencil },
  permissions: {
    labelKey: "environments.permissionsCardTitle",
    icon: LockKeyhole,
  },
  "cross-environment-connections": {
    labelKey: "environments.connectionsNavLabel",
    icon: ShieldOff,
  },
  "environment-groups": {
    labelKey: "environments.envGroupsCardTitle",
    icon: Layers,
  },
  networking: { labelKey: "environments.networkingCardTitle", icon: Network },
  "delete-environment": {
    labelKey: "environments.deleteCardTitle",
    icon: Trash2,
  },
};

/** The page's in-page navigation, shared with its pending frame. */
export function EnvironmentSettingsNavigation({
  sections,
}: {
  sections: EnvironmentSettingsSection[];
}) {
  const { t } = useTranslations();
  return (
    <SectionNavigation
      ariaLabel={t("environments.settingsNavigation")}
      items={sections.map(
        (section): SectionNavigationItem => ({
          href: `#${section}`,
          label: t(SECTION_COPY[section].labelKey),
          icon: SECTION_COPY[section].icon,
        }),
      )}
    />
  );
}

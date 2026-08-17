import {
  Activity,
  Info,
  MemoryStick,
  Network,
  Package,
  Plug,
  TriangleAlert,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the key-value detail page's Overview tab —
 * same right-rail quick nav as the service settings page. Labels reuse the
 * cards' own title keys. Rendered only on the Overview tab; the Logs tab is a
 * single viewer and gets no nav.
 */
export function KeyValueDetailNavigation({
  className,
}: {
  className?: string;
}) {
  const { t } = useTranslations();
  const items = [
    { href: "#metadata", label: t("keyvalue.metaTitle"), icon: Info },
    { href: "#connection", label: t("keyvalue.connTitle"), icon: Plug },
    {
      href: "#networking",
      label: t("keyvalue.networkingTitle"),
      icon: Network,
    },
    { href: "#plan", label: t("keyvalue.planTitle"), icon: Package },
    {
      href: "#maxmemory-policy",
      label: t("keyvalue.maxmemoryTitle"),
      icon: MemoryStick,
    },
    {
      href: "#metrics",
      label: t("metrics.datastoreMetricsTitle"),
      icon: Activity,
    },
    {
      href: "#danger-zone",
      label: t("keyvalue.dangerZoneTitle"),
      icon: TriangleAlert,
    },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("keyvalue.sectionNavigation")}
      items={items}
      className={className}
    />
  );
}

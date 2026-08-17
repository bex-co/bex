import {
  Activity,
  History,
  Info,
  Lightbulb,
  Package,
  Plug,
  Server,
  ShieldCheck,
  Terminal,
  TriangleAlert,
} from "lucide-react";
import { SectionNavigation } from "@/common/components/section-navigation";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * In-page section navigation for the database detail page's Overview tab —
 * same right-rail quick nav as the service settings page. Labels reuse the
 * cards' own title keys. Rendered only on the Overview tab; the Logs tab is a
 * single viewer and gets no nav.
 */
export function DatabaseDetailNavigation({
  className,
}: {
  className?: string;
}) {
  const { t } = useTranslations();
  const items = [
    { href: "#metadata", label: t("databases.metaTitle"), icon: Info },
    { href: "#connection", label: t("databases.connTitle"), icon: Plug },
    { href: "#sql-console", label: t("databases.sqlTitle"), icon: Terminal },
    { href: "#high-availability", label: t("databases.haTitle"), icon: Server },
    {
      href: "#metrics",
      label: t("metrics.datastoreMetricsTitle"),
      icon: Activity,
    },
    { href: "#plan", label: t("databases.planTitle"), icon: Package },
    { href: "#insights", label: t("databases.insightsTitle"), icon: Lightbulb },
    { href: "#recovery", label: t("databases.recoveryTitle"), icon: History },
    {
      href: "#access-control",
      label: t("databases.accessTitle"),
      icon: ShieldCheck,
    },
    {
      href: "#danger-zone",
      label: t("databases.dangerZoneTitle"),
      icon: TriangleAlert,
    },
  ];

  return (
    <SectionNavigation
      ariaLabel={t("databases.sectionNavigation")}
      items={items}
      className={className}
    />
  );
}

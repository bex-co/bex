import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import type { en } from "@/i18n";

// bex's subset of Render's service sidebar (docs .pm/w5/m5 "Render reference"):
// an Overview landing (bex addition — Render's root lands on Events/deploys), the
// Environment tab (env vars, w4/m6.5), the Monitor-group Logs + Metrics items,
// and Settings (Instance Type, w5/m7). Events comes later. `exact` keeps
// Overview from staying active on the nested routes.
interface ServiceNavItem {
  labelKey: keyof typeof en;
  to: string;
  exact: boolean;
}

const ITEMS: ServiceNavItem[] = [
  { labelKey: "services.navOverview", to: "/services/$serviceId", exact: true },
  {
    labelKey: "services.navEvents",
    to: "/services/$serviceId/events",
    exact: false,
  },
  {
    labelKey: "services.navEnvironment",
    to: "/services/$serviceId/env",
    exact: false,
  },
  {
    labelKey: "services.navLogs",
    to: "/services/$serviceId/logs",
    exact: false,
  },
  {
    labelKey: "services.navMetrics",
    to: "/services/$serviceId/metrics",
    exact: false,
  },
  {
    labelKey: "services.navScaling",
    to: "/services/$serviceId/scaling",
    exact: false,
  },
  {
    labelKey: "services.navSettings",
    to: "/services/$serviceId/settings",
    exact: false,
  },
];

export function ServiceNav({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();

  return (
    <nav
      aria-label={t("services.navLabel")}
      className="flex gap-1 border-b px-2 sm:px-4"
    >
      {ITEMS.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          params={{ serviceId }}
          activeOptions={{ exact: item.exact }}
          className={cn(
            "border-b-2 border-transparent px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground",
            "data-[status=active]:border-foreground data-[status=active]:text-foreground",
          )}
        >
          {t(item.labelKey)}
        </Link>
      ))}
    </nav>
  );
}

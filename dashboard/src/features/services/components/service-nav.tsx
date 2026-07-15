import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import type { en } from "@/i18n";

// bex's subset of Render's service sidebar (docs .pm/w5/m5 "Render reference"):
// Events is the landing tab (Render's service root lands on Events/deploys too —
// there is no Overview page; the identity facts live in the detail header), then
// the Environment tab (env vars, w4/m6.5), the Monitor-group Logs + Metrics
// items, Scaling, and Settings (Instance Type, w5/m7).
interface ServiceNavItem {
  labelKey: keyof typeof en;
  to: string;
  exact: boolean;
}

const ITEMS: ServiceNavItem[] = [
  {
    labelKey: "services.navEvents",
    to: "/services/$serviceId/events",
    exact: false,
  },
  // The dedicated deploy-history tab (w9/002) — Render's standalone Deploys
  // list. exact:false keeps it highlighted on the nested per-deploy pages
  // (/deploys/$deployId).
  {
    labelKey: "services.navDeploys",
    to: "/services/$serviceId/deploys",
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
      className="flex snap-x gap-1 overflow-x-auto border-b px-2 [scrollbar-width:none] sm:px-4 [&::-webkit-scrollbar]:hidden"
    >
      {ITEMS.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          params={{ serviceId }}
          activeOptions={{ exact: item.exact }}
          className={cn(
            "shrink-0 snap-start whitespace-nowrap border-b-2 border-transparent px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground",
            "data-[status=active]:border-foreground data-[status=active]:text-foreground",
          )}
        >
          {t(item.labelKey)}
        </Link>
      ))}
    </nav>
  );
}

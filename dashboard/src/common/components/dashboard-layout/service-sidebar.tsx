import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronLeft } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from "@/common/components/ui/sidebar.tsx";
import { serviceNavGroups } from "@/features/services/components/service-nav";
import { deriveServiceType } from "@/features/services/lib/service-type";
import { useServiceBase } from "@/features/services/lib/service-base";
import { useServer } from "@/features/services/hooks/use-server";
import { isNavItemActive } from "./nav-active";
import { SidebarBrand } from "./sidebar-brand";
import { SidebarNavGroups } from "./sidebar-nav-groups";

export interface ServiceSidebarProps {
  serviceId: string;
}

/**
 * Render parity (w1/m45): a service's pages swap the global workspace nav for
 * a resource-scoped sidebar — a back link to the workspace overview, the
 * service name, then the service's own navigation in Render's grouping
 * (top-level items, Monitor, Manage — the item list lives in
 * features/services/components/service-nav.tsx, the single source of truth).
 * Same contextual-sidebar pattern the project pages established
 * (project-sidebar.tsx); the logo + workspace switcher stay pinned on top.
 *
 * Reads the service through `useServer(serviceId)` — the same document +
 * variables the detail layout already fetches, so Apollo dedupes this to zero
 * extra requests AND the sidebar's existence signal agrees with the shell's
 * not-found state by construction.
 */
export function ServiceSidebar({ serviceId }: ServiceSidebarProps) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const base = useServiceBase();
  const { t } = useTranslations();
  const { service, loading } = useServer(serviceId);
  // An id the caller can't see gets no service nav — the shell renders its
  // not-found state, and a sidebar of dead links would contradict it. While
  // still loading, keep the nav (a legit id must not flash empty→full).
  const showNav = loading || service !== null;

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent>
        <div className="flex flex-col gap-2 px-2 pt-2 pb-1 group-data-[collapsible=icon]:hidden">
          <Link
            to="/"
            className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
          >
            <ChevronLeft className="size-3.5" />
            {t("common.navBackToDashboard")}
          </Link>
          <p className="truncate text-sm font-semibold">
            {service?.name ?? serviceId}
          </p>
        </div>
        {showNav ? (
          <SidebarNavGroups
            groups={serviceNavGroups(
              service ? deriveServiceType(service.type) : null,
              base,
            )}
            linkParams={{ serviceId }}
            isItemActive={(to) =>
              isNavItemActive(pathname, to.replace("$serviceId", serviceId))
            }
          />
        ) : null}
      </SidebarContent>
    </Sidebar>
  );
}

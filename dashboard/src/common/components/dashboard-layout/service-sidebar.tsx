import { useRouterState } from "@tanstack/react-router";
import { serviceNavGroups } from "@/features/services/components/service-nav";
import { deriveServiceType } from "@/features/services/lib/service-type";
import { useServiceBase } from "@/features/services/lib/service-base";
import { useServer } from "@/features/services/hooks/use-server";
import {
  useCachedServiceName,
  useCachedServiceType,
} from "@/features/services/hooks/use-cached-service-type";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { isNavItemActive } from "./nav-active";
import { ResourceSidebar } from "./resource-sidebar";
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
  const { service, loading } = useServer(serviceId, { poll: false });
  // An id the caller can't see gets no service nav — the shell renders its
  // not-found state, and a sidebar of dead links would contradict it. While
  // still loading, keep the nav (a legit id must not flash empty→full).
  const showNav = loading || service !== null;
  // While the read loads, the cache the header's frame reads (the workspace's
  // service list, after a click from it) already names and types the
  // service, so the title and the nav are the ready page's from the first
  // frame (w3/119). With nothing cached the nav stays type-less: a static
  // site never flashes Deploys.
  const pending = loading && service === null;
  const cachedName = useCachedServiceName(serviceId);
  const cachedType = useCachedServiceType(serviceId);
  const name = service?.name ?? (pending ? cachedName : null) ?? serviceId;
  const type = service?.type ?? (pending ? cachedType : null);
  // A Billing member views no service events, deploys or metrics (w9/m621),
  // and neither a Viewer nor a Billing member views logs (w9/145).
  const { denied } = useCapabilities();
  const activityDenied = denied("can_view_activity");
  const logsDenied = denied("can_view_logs");

  return (
    <ResourceSidebar name={name}>
      {showNav ? (
        <SidebarNavGroups
          groups={serviceNavGroups(
            type ? deriveServiceType(type) : null,
            base,
            { activityDenied, logsDenied },
          )}
          linkParams={{ serviceId }}
          isItemActive={(to) =>
            isNavItemActive(pathname, to.replace("$serviceId", serviceId))
          }
        />
      ) : null}
    </ResourceSidebar>
  );
}

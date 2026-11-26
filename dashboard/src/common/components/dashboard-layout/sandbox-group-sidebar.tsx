import { useRouterState } from "@tanstack/react-router";
import {
  isSandboxGroupNavItemActive,
  SANDBOX_GROUP_NAV_GROUPS,
} from "@/features/sandbox-group-pages/lib/nav";
import { useSandboxGroup } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import { ResourceSidebar } from "./resource-sidebar";
import { SidebarNavGroups } from "./sidebar-nav-groups";

/**
 * Render parity (w1/m658): a sandbox group's pages swap the workspace nav for
 * the group's own sidebar, as Render's do: Sandboxes, Snapshots, Custom
 * images, then Settings (features/sandbox-group-pages/lib/nav.ts). It reads
 * the group through the same document and variables as the group layout, so
 * Apollo serves it from the cache.
 */
export function SandboxGroupSidebar({ sandboxGroupId }: { sandboxGroupId: string }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { group, loading } = useSandboxGroup(sandboxGroupId);
  // An id the caller can't see gets no nav: the layout redirects it home.
  const showNav = loading || group !== null;
  return (
    <ResourceSidebar name={group?.name ?? sandboxGroupId}>
      {showNav ? (
        <SidebarNavGroups
          groups={[...SANDBOX_GROUP_NAV_GROUPS]}
          linkParams={{ sandboxGroupId }}
          isItemActive={(to) =>
            isSandboxGroupNavItemActive(pathname, sandboxGroupId, to)
          }
        />
      ) : null}
    </ResourceSidebar>
  );
}

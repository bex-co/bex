import { useRouterState } from "@tanstack/react-router";
import {
  databaseNavGroups,
  isDatabaseNavItemActive,
} from "@/features/databases/components/database-nav";
import { useDatabase } from "@/features/databases/hooks/use-database";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { ResourceSidebar } from "./resource-sidebar";
import { SidebarNavGroups } from "./sidebar-nav-groups";

/**
 * Render parity (w3/m641): a Postgres database's pages swap the workspace nav
 * for a resource sidebar, as a service's do (service-sidebar.tsx): Info /
 * Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery (the list lives in
 * features/databases/components/database-nav.tsx).
 *
 * Reads the database through `useDatabase(databaseId)` — the same document
 * and variables the detail layout reads, cache-first and without a second
 * poll, so Apollo serves it with no extra request and the sidebar agrees with
 * the layout on whether the database exists. The entries never depend on the
 * database itself, so the loading sidebar already draws them all.
 */
export function DatabaseSidebar({ databaseId }: { databaseId: string }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { database, loading } = useDatabase(databaseId, { poll: false });
  // An id the caller can't see gets no nav — the layout renders its
  // not-found state, and a sidebar of dead links would contradict it.
  const showNav = loading || database !== null;
  const { denied } = useCapabilities();

  return (
    <ResourceSidebar name={database?.name ?? databaseId}>
      {showNav ? (
        <SidebarNavGroups
          groups={databaseNavGroups({
            logsDenied: denied("can_view_logs"),
            metricsDenied: denied("can_view_activity"),
          })}
          linkParams={{ databaseId }}
          isItemActive={(to) =>
            isDatabaseNavItemActive(pathname, databaseId, to)
          }
        />
      ) : null}
    </ResourceSidebar>
  );
}

import { useRouterState } from "@tanstack/react-router";
import {
  isKeyValueNavItemActive,
  keyValueNavGroups,
} from "@/features/keyvalue/components/key-value-nav";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { ResourceSidebar } from "./resource-sidebar";
import { SidebarNavGroups } from "./sidebar-nav-groups";

/**
 * Render parity (w3/m649): a Key Value instance's pages swap the workspace nav
 * for a resource sidebar, as a Postgres database's do (database-sidebar.tsx):
 * Info / Monitor: Logs, Metrics (the list lives in
 * features/keyvalue/components/key-value-nav.tsx).
 *
 * Reads the instance through `useKeyValue(keyValueId)` — the same document
 * and variables the detail layout reads, cache-first and without a second
 * poll, so Apollo serves it with no extra request and the sidebar agrees with
 * the layout on whether the instance exists. The entries never depend on the
 * instance itself, so the loading sidebar already draws them all.
 */
export function KeyValueSidebar({ keyValueId }: { keyValueId: string }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { keyValue, loading } = useKeyValue(keyValueId, { poll: false });
  // An id the caller can't see gets no nav — the layout redirects it home,
  // and a sidebar of dead links would contradict it.
  const showNav = loading || keyValue !== null;
  const { denied } = useCapabilities();

  return (
    <ResourceSidebar name={keyValue?.name ?? keyValueId}>
      {showNav ? (
        <SidebarNavGroups
          groups={keyValueNavGroups({
            logsDenied: denied("can_view_logs"),
            metricsDenied: denied("can_view_activity"),
          })}
          linkParams={{ keyValueId }}
          isItemActive={(to) =>
            isKeyValueNavItemActive(pathname, keyValueId, to)
          }
        />
      ) : null}
    </ResourceSidebar>
  );
}

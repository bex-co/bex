import { useState } from "react";
import { useServices } from "@/features/services/hooks/use-services";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import type { ProjectResourceCatalog } from "@/features/projects/lib/resource-filter";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * The three workspace resource lists a Project Overview joins its membership
 * against, plus whether each has answered (w4/m227). An unread or failed list
 * is unknown membership, never an empty one, so its environment must not say
 * "0 resources"; `catalog.retry` re-reads only the failed lists.
 */
export function useProjectResourceCatalog() {
  const services = useServices();
  const databases = useDatabases();
  const keyValues = useKeyValues();
  const families = [
    ["services", services],
    ["databases", databases],
    ["keyvalues", keyValues],
  ] as const;
  const catalog: ProjectResourceCatalog = {
    pending: families.filter(([, list]) => list.loading).map(([kind]) => kind),
    failed: families.filter(([, list]) => list.error).map(([kind]) => kind),
    retry: () =>
      Promise.all(
        families.map(([, list]) => (list.error ? list.refetch() : null)),
      ),
  };
  const { currentWorkspaceId } = useWorkspace();
  return {
    services: useLastGoodList(currentWorkspaceId, services.services, services),
    databases: useLastGoodList(
      currentWorkspaceId,
      databases.databases,
      databases,
    ),
    keyValues: useLastGoodList(
      currentWorkspaceId,
      keyValues.keyValues,
      keyValues,
    ),
    refetchServices: services.refetch,
    refetchDatabases: databases.refetch,
    refetchKeyValues: keyValues.refetch,
    catalog,
  };
}

/**
 * A list as last answered for this workspace while a refresh of it fails: the
 * hooks map a failed read to [], which would empty an environment that the
 * alert already marks as possibly out of date. Another workspace's list is
 * never kept. Adjusted during render, React's pattern for derived state.
 */
function useLastGoodList<T>(
  workspaceId: string | null,
  list: T[],
  read: { loading: boolean; error?: unknown },
): T[] {
  const answered = !read.loading && !read.error;
  const [kept, setKept] = useState<{
    workspaceId: string | null;
    list: T[];
  } | null>(null);
  if (
    answered &&
    (kept?.workspaceId !== workspaceId || !sameItems(kept.list, list))
  ) {
    setKept({ workspaceId, list });
  }
  if (answered || kept?.workspaceId !== workspaceId) return list;
  return kept.list;
}

function sameItems<T>(a: T[], b: T[]): boolean {
  return a.length === b.length && a.every((item, i) => item === b[i]);
}

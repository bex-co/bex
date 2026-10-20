import { useCallback } from "react";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { environmentNameIn } from "@/features/env-groups/lib/scope";
import { strandedGroupsForDestination } from "@/features/env-groups/lib/stranded";
import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The move-time side of w4/m238: before a dashboard move writes, ask which
 * linked groups it would strand (see lib/stranded). `ready` is false while
 * either read is unresolved — a move must not slip silently past an unknown,
 * so callers block and surface the retry instead of proceeding.
 */
export function useStrandedLinkGuard() {
  const { groups, loading: groupsLoading } = useEnvGroups();
  const scope = useEnvGroupScopeIndex();

  const strandedFor = useCallback(
    (serviceIds: ReadonlyArray<string>, destinationEnvironmentId: string | null) =>
      strandedGroupsForDestination(groups, serviceIds, destinationEnvironmentId),
    [groups],
  );

  /** "<project> / <environment>" for a group's scope; undefined for workspace
   *  scope or a scope the index does not hold. */
  const scopeNameFor = useCallback(
    (group: Pick<EnvGroupView, "environmentId">): string | undefined =>
      group.environmentId == null
        ? undefined
        : environmentNameIn(scope, group.environmentId),
    // environmentNameIn reads only byId + projects off the index.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [scope.byId, scope.projects],
  );

  return {
    groups,
    ready: scope.ready && !groupsLoading,
    loading: scope.loading || groupsLoading,
    error: scope.error,
    retry: scope.retry,
    strandedFor,
    scopeNameFor,
  };
}

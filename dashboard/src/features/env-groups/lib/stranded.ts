import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The groups a service move would strand: linked to one of the moving
 * services, but scoped somewhere other than the destination (w4/m238).
 *
 * Scope is exact equality, workspace included — a group scoped to an
 * environment (`environmentId` set) strands when the service lands anywhere
 * else, and a workspace-scoped group (`null`) strands when the service lands
 * in an environment. This is `serviceMatchesGroupScope` with the service's
 * post-move environment substituted for its current one. Moves stay allowed —
 * the group-side guard's own remedy depends on them — so callers confirm
 * rather than refuse.
 */
export function strandedGroupsForDestination(
  groups: ReadonlyArray<EnvGroupView>,
  serviceIds: ReadonlyArray<string>,
  destinationEnvironmentId: string | null,
): EnvGroupView[] {
  const moving = new Set(serviceIds);
  return groups.filter(
    (group) =>
      group.environmentId !== destinationEnvironmentId &&
      group.serviceLinks.some((serviceId) => moving.has(serviceId)),
  );
}

/** One stranded group as the confirmation dialog lists it. */
export interface StrandedPromptItem {
  id: string;
  name: string;
  /** "<project> / <environment>", or undefined for a workspace-scoped group. */
  scopeLabel?: string;
}

/** Maps stranded groups to dialog items; `scopeNameFor` resolves a group's
 *  "<project> / <environment>" label (undefined = workspace scope). */
export function strandedPromptItems(
  stranded: ReadonlyArray<EnvGroupView>,
  scopeNameFor: (group: EnvGroupView) => string | undefined,
): StrandedPromptItem[] {
  return stranded.map((group) => ({
    id: group.id,
    name: group.name,
    scopeLabel: scopeNameFor(group),
  }));
}

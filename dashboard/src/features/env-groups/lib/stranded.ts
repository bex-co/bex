import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The groups a service move would strand: linked to one of the moving
 * services, but scoped somewhere other than the destination (w4/m238).
 *
 * A group scoped to an environment (`environmentId` set) strands when the
 * service lands anywhere else; a workspace-scoped group (`null`) links to any
 * service, so it never strands (w8/179). This is `serviceMatchesGroupScope` with the service's
 * post-move environment substituted for its current one. Moves stay allowed —
 * the group-side guard's own remedy depends on them — so callers confirm
 * rather than refuse.
 */
export function strandedGroupsForDestination(
  groups: ReadonlyArray<EnvGroupView>,
  serviceIds: ReadonlyArray<string>,
  destinationEnvironmentId: string | null,
  comoving?: CoMovingGroups,
): EnvGroupView[] {
  const moving = new Set(serviceIds);
  const groupIds = new Set(comoving?.groupIds);
  const landed = new Set([
    ...serviceIds,
    ...(comoving?.destinationServiceIds ?? []),
  ]);
  // A group moving with the services is in scope after the move — but only
  // if every service it links lands there too: bex-api refuses a group move
  // that would leave one behind (ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES), and
  // the moving services would then stay linked across environments (w4/564).
  const movesAlong = (group: EnvGroupView) =>
    groupIds.has(group.id) && group.serviceLinks.every((id) => landed.has(id));
  return groups.filter(
    (group) =>
      group.environmentId !== null &&
      group.environmentId !== destinationEnvironmentId &&
      !movesAlong(group) &&
      group.serviceLinks.some((serviceId) => moving.has(serviceId)),
  );
}

/** The groups that move to the destination in the same move (w4/564). */
export interface CoMovingGroups {
  groupIds: ReadonlyArray<string>;
  /** Services already at the destination, which a moving group may link. */
  destinationServiceIds: ReadonlyArray<string>;
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

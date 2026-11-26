import { scopeAdmits } from "@/features/env-groups/lib/scope";
import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The groups a new service in `environmentId` can link, in the order Render's
 * create form lists them (docs/render-artifacts/new-service-wizard.md § The
 * Advanced section): the workspace's own groups by name, then the
 * Environment's by name. A service in no Environment links workspace groups
 * only, the rule bex-api enforces (`ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`).
 */
export function linkableEnvGroups(
  groups: ReadonlyArray<EnvGroupView>,
  environmentId: string | null,
): { workspace: EnvGroupView[]; environment: EnvGroupView[] } {
  const linkable = groups
    .filter((group) => scopeAdmits(group.environmentId, environmentId))
    .sort((a, b) => a.name.localeCompare(b.name));
  return {
    workspace: linkable.filter((group) => group.environmentId === null),
    environment: linkable.filter((group) => group.environmentId !== null),
  };
}

/** The ids of {@link linkableEnvGroups}. */
export function linkableEnvGroupIds(
  groups: ReadonlyArray<EnvGroupView>,
  environmentId: string | null,
): Set<string> {
  const { workspace, environment } = linkableEnvGroups(groups, environmentId);
  return new Set([...workspace, ...environment].map((group) => group.id));
}

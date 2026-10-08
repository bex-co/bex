import type { EnvGroupView } from "@/features/env-groups/types";

/** The fields environmentScopeName reads. */
type NamedEnvironment = { name: string; projectId: string };
type NamedProject = { id: string; name: string };

/**
 * "Project / Environment" for an Environment scope. Environment names are
 * unique within a project only, so a workspace with two "Production"s showed
 * two identical choices and an ambiguous saved scope (w4/215). Falls back to
 * the bare name when the project is not in the authorized index.
 */
export function environmentScopeName(
  environment: NamedEnvironment,
  projects: ReadonlyArray<NamedProject>,
): string {
  const project = projects.find((item) => item.id === environment.projectId);
  return project ? `${project.name} / ${environment.name}` : environment.name;
}

/**
 * The Select value standing in for "no Environment". A `Select` cannot hold an
 * empty-string item value, so the workspace scope needs a sentinel; the
 * helpers below translate it back to `null` on the wire.
 */
export const WORKSPACE_SCOPE = "__workspace__";

/** environmentScopeName for a saved scope id, or undefined when the index
 *  does not hold it. */
export function environmentNameIn(
  index: {
    byId: ReadonlyMap<string, NamedEnvironment>;
    projects: ReadonlyArray<NamedProject>;
  },
  environmentId: string,
): string | undefined {
  const environment = index.byId.get(environmentId);
  return environment
    ? environmentScopeName(environment, index.projects)
    : undefined;
}

/** `null` (workspace) -> the sentinel the Select needs. */
export function scopeValue(environmentId: string | null): string {
  return environmentId || WORKSPACE_SCOPE;
}

/** The Select's value back to the wire's `environmentId`. */
export function scopeEnvironmentId(value: string): string | null {
  return value === WORKSPACE_SCOPE ? null : value;
}

/**
 * Whether a service may be linked to a group in `environmentId`. bex-api
 * refuses a cross-scope link (`ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH`), so
 * every surface that offers a link — the detail page's picker and, since
 * w4/m111, the create dialog's checkbox list — filters on exactly this rule.
 * A service in no environment is workspace-scoped, which is `null`.
 */
export function serviceMatchesScope(
  serviceEnvironmentById: ReadonlyMap<string, string>,
  serviceId: string,
  environmentId: string | null,
): boolean {
  return (serviceEnvironmentById.get(serviceId) ?? null) === environmentId;
}

/** `serviceMatchesScope` bound to a group's own scope. */
export function serviceMatchesGroupScope(
  serviceEnvironmentById: ReadonlyMap<string, string>,
  serviceId: string,
  group: Pick<EnvGroupView, "environmentId">,
): boolean {
  return serviceMatchesScope(
    serviceEnvironmentById,
    serviceId,
    group.environmentId,
  );
}

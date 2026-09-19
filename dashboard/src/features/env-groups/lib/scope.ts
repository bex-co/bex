import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The Select value standing in for "no Environment". A `Select` cannot hold an
 * empty-string item value, so the workspace scope needs a sentinel; the
 * helpers below translate it back to `null` on the wire.
 */
export const WORKSPACE_SCOPE = "__workspace__";

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

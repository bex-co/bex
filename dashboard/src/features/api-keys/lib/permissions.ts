// The permission picker's model (w9/m496, Render's scoped API keys,
// render.com/docs/api-keys read 2026-11-18). Every function here reads the
// catalog bex-api sends; none of them knows a permission by name.

import type { ApiKeyPermissionsInput } from "@/graphql/definitions";
import type {
  ApiKeyPermissionsView,
  MemberRole,
  PermissionAreaView,
  PermissionLevel,
  PermissionSelection,
  PermissionView,
} from "@/features/api-keys/types";

/** The level Render's Read only preset grants. */
export const READ_LEVEL: PermissionLevel = "read";

const PERMISSION_LEVELS = new Set<string>([
  "read",
  "write",
  "read_write",
  "run",
  "execute",
] satisfies PermissionLevel[]);

const MEMBER_ROLES = new Set<string>([
  "viewer",
  "contributor",
  "developer",
  "admin",
] satisfies MemberRole[]);

/** Narrows bex-api's level string to the levels the picker knows. */
export function isPermissionLevel(level: string): level is PermissionLevel {
  return PERMISSION_LEVELS.has(level);
}

function isMemberRole(role: string): role is MemberRole {
  return MEMBER_ROLES.has(role);
}

/** Narrows bex-api's role string; null for a role the picker does not know. */
export function toMemberRole(role: string): MemberRole | null {
  return isMemberRole(role) ? role : null;
}

function permissionsOf(catalog: PermissionAreaView[]): PermissionView[] {
  return catalog.flatMap((area) => area.permissions);
}

/**
 * The broadest level of `permission` the viewer may grant, or null when they
 * may grant none. bex-api lists a permission's levels in catalog order, whose
 * last level grants every action the others do (a backend test pins it), so
 * the broadest grantable one is the last grantable one.
 */
export function highestGrantableLevel(
  permission: PermissionView,
): PermissionLevel | null {
  return permission.levels.filter((l) => l.grantable).at(-1)?.level ?? null;
}

/** All: every permission at its broadest grantable level. */
function allLevels(
  catalog: PermissionAreaView[],
): Record<string, PermissionLevel> {
  const levels: Record<string, PermissionLevel> = {};
  for (const permission of permissionsOf(catalog)) {
    const level = highestGrantableLevel(permission);
    if (level) levels[permission.id] = level;
  }
  return levels;
}

/** The All preset. */
export const ALL_SELECTION: PermissionSelection = { all: true, levels: {} };

/** Clear all: every permission None. */
export const CLEARED_SELECTION: PermissionSelection = {
  all: false,
  levels: {},
};

/**
 * Read only: "Grants Read access to every applicable resource" — each
 * permission's Read level where it offers one the viewer may grant, else None.
 */
export function readOnlySelection(
  catalog: PermissionAreaView[],
): PermissionSelection {
  const levels: Record<string, PermissionLevel> = {};
  for (const permission of permissionsOf(catalog)) {
    const read = permission.levels.find((l) => l.level === READ_LEVEL);
    if (read?.grantable) levels[permission.id] = READ_LEVEL;
  }
  return { all: false, levels };
}

/** The level each permission shows: All's broadest levels, or the choice. */
export function effectiveLevels(
  selection: PermissionSelection,
  catalog: PermissionAreaView[],
): Record<string, PermissionLevel> {
  return selection.all ? allLevels(catalog) : selection.levels;
}

/**
 * The selection with one permission set to `level` (null is None). An edit
 * after All starts from what All showed, so only that row changes.
 */
export function withLevel(
  selection: PermissionSelection,
  catalog: PermissionAreaView[],
  permissionId: string,
  level: PermissionLevel | null,
): PermissionSelection {
  const levels = { ...effectiveLevels(selection, catalog) };
  if (level) levels[permissionId] = level;
  else delete levels[permissionId];
  return { all: false, levels };
}

/**
 * An area's toggle: on grants each of its permissions at its broadest
 * grantable level (All, for that area), off sets each to None.
 */
export function withArea(
  selection: PermissionSelection,
  catalog: PermissionAreaView[],
  area: PermissionAreaView,
  on: boolean,
): PermissionSelection {
  const levels = { ...effectiveLevels(selection, catalog) };
  for (const permission of area.permissions) {
    const level = on ? highestGrantableLevel(permission) : null;
    if (level) levels[permission.id] = level;
    else delete levels[permission.id];
  }
  return { all: false, levels };
}

/**
 * What the mutation sends: `{all: true}` for an untouched All, otherwise the
 * chosen levels in the catalog's order (`{grants: []}` is Clear all).
 */
export function selectionToInput(
  selection: PermissionSelection,
  catalog: PermissionAreaView[],
): ApiKeyPermissionsInput {
  if (selection.all) return { all: true };
  return {
    grants: permissionsOf(catalog)
      .filter((permission) => selection.levels[permission.id])
      .map((permission) => ({
        permission: permission.id,
        level: selection.levels[permission.id],
      })),
  };
}

/** A key's grant as the picker's starting value. */
export function selectionFromPermissions(
  permissions: ApiKeyPermissionsView,
): PermissionSelection {
  if (permissions.all) return ALL_SELECTION;
  return {
    all: false,
    levels: Object.fromEntries(
      permissions.grants.map((grant) => [grant.permission, grant.level]),
    ),
  };
}

/** Whether two selections grant the same thing. */
export function sameSelection(
  a: PermissionSelection,
  b: PermissionSelection,
): boolean {
  if (a.all || b.all) return a.all === b.all;
  const keys = Object.keys(a.levels);
  return (
    keys.length === Object.keys(b.levels).length &&
    keys.every((key) => a.levels[key] === b.levels[key])
  );
}

/** A key row's one-word grant, and how many permissions a custom one names. */
export type PermissionsSummary =
  | { kind: "all" }
  | { kind: "readOnly"; count: number }
  | { kind: "none" }
  | { kind: "custom"; count: number };

/**
 * The row summary: All; No permissions (Clear all); Read only when every
 * permission it names is at Read, so the key can change nothing; else Custom.
 * Both name how many permissions the key holds, so a key with one Read
 * permission does not read as Render's every-resource Read only preset.
 */
export function summarizePermissions(
  permissions: ApiKeyPermissionsView,
): PermissionsSummary {
  if (permissions.all) return { kind: "all" };
  const { grants } = permissions;
  if (grants.length === 0) return { kind: "none" };
  if (grants.every((grant) => grant.level === READ_LEVEL)) {
    return { kind: "readOnly", count: grants.length };
  }
  return { kind: "custom", count: grants.length };
}

// bex-native projection of bex-api's `ApiKey` (backend/internal/apikeys —
// machine credentials, workspace-shared, not per-user; docs/ADR012-auth.md). The list
// never carries the key or its secret (the server always resolves them empty
// for `apiKeys`, and the dashboard's list query doesn't even request the fields).

export interface ApiKeyView {
  id: string;
  name: string;
  createdAt: string | null;
  /** Identity subject that minted the key (client_id or Kratos id); null if unknown. */
  createdBy: string | null;
  /** RFC 3339 of the last time the key was used; null if never. */
  lastUsedAt: string | null;
  /** What the key may do (w9/m496), as Render's scoped keys carry. */
  permissions: ApiKeyPermissionsView;
}

/** One permission of a key's grant at one of the levels it offers. */
export interface ApiKeyGrantView {
  /** A catalog permission id (`services`, `services_env`, …). */
  permission: string;
  level: PermissionLevel;
}

/**
 * A key's grant. `all` is Render's All: every permission its creator's member
 * role allows, following the role. Otherwise exactly `grants`; none is Clear
 * all (the key can only identify itself).
 */
export interface ApiKeyPermissionsView {
  all: boolean;
  grants: ApiKeyGrantView[];
}

/** A freshly minted key — the one and only time `key` and `secret` are available. */
export interface CreatedApiKey {
  /** The key's id, which is also its OAuth2 client_id. */
  id: string;
  name: string;
  /** The bearer string (`bex_<id>_<secret>`) sent as `Authorization: Bearer`. */
  key: string;
  /** The OAuth2 client_secret, for the optional client_credentials exchange. */
  secret: string;
}

// The permission picker's catalog (w9/m496): bex-api's
// `apiKeyPermissionCatalog`, Render's tables with, per level, whether the
// viewer's role may grant it. The dashboard holds no copy of it.

/**
 * Render's permission levels. bex-api sends them as strings; the hooks narrow
 * them here (`isPermissionLevel`) and drop a value outside this set.
 */
export type PermissionLevel =
  "read" | "write" | "read_write" | "run" | "execute";

/** The member roles a level can require, lowest first. */
export type MemberRole = "viewer" | "contributor" | "developer" | "admin";

/** One action a level grants; `bex` marks one Render's table does not list. */
export interface PermissionActionView {
  id: string;
  name: string;
  bex: boolean;
}

/** One level a permission offers. */
export interface PermissionLevelView {
  level: PermissionLevel;
  /** Render's display name: "Read", "Write", "Read & write", "Run", "Execute". */
  name: string;
  /** Whether the viewer's current member role may grant this level. */
  grantable: boolean;
  /** The lowest member role that may; null for one this dashboard does not know. */
  requiredRole: MemberRole | null;
  actions: PermissionActionView[];
}

export interface PermissionView {
  id: string;
  name: string;
  levels: PermissionLevelView[];
}

/** One of Render's resource types, or a bex area (`bex`) after them. */
export interface PermissionAreaView {
  id: string;
  name: string;
  bex: boolean;
  permissions: PermissionView[];
}

/**
 * The picker's value. `all` is the All preset, shown as every permission at
 * its highest grantable level and sent as `{all: true}`; any edit after it
 * turns into an explicit selection. Otherwise `levels` maps a permission id to
 * its chosen level, and a permission absent from it is None.
 */
export interface PermissionSelection {
  all: boolean;
  levels: Record<string, PermissionLevel>;
}

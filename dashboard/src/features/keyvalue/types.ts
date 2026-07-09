// bex-native projection of a managed Key Value (Valkey) row, mapped from
// bex-api's Render dashboard-shaped GraphQL `KeyValue`
// (backend/internal/keyvalue/graphql.go). The wire type carries Render's
// `status` enum (available/creating/unavailable) and a string `suspended`
// enum; this view normalizes what the UI renders so pages never re-derive
// Render's encoding (mirrors databases' DatabaseView). Unlike Database, there
// is no separate detail-only field set — `keyValue(id)` returns the same
// shape as the list, so one view covers both.

export interface KeyValueView {
  /** Key Value store name — Render's opaque id; also the detail-page deep-link param. */
  id: string;
  name: string;
  /** Render's keyValueStatus enum, verbatim (available/creating/unavailable). */
  status: string;
  /** Plan spelling from the tiers catalog (e.g. "free"), or null. */
  plan: string | null;
  /** Valkey version, or null when the operator default was used. */
  version: string | null;
  createdAt: string | null;
  /** SNI host for the external endpoint, or null when private. */
  externalHost: string | null;
  /** Whether the external (public) endpoint is enabled. */
  public: boolean;
  /** Derived from Render's string `suspended` enum (services/lib/status's isSuspended), not a raw boolean field. */
  suspended: boolean;
}

/** The connection strings, fetched on demand (never in the list). The
 * password lives inside the `redis://` URI itself — unlike Postgres, there is
 * no standalone password field. */
export interface KeyValueConnectionInfoView {
  internalConnectionString: string;
  externalConnectionString: string;
  cliCommand: string;
}

/** One catalog plan as the create form's picker consumes it. */
export interface KeyValueInstanceTypeView {
  /** spec.plan spelling — what createKeyValue accepts. */
  id: string;
  name: string;
  cpu: string;
  memory: string;
  storageGB: number;
}

/** A resolved status key (i18n label) + the badge variant it renders as. */
export type KeyValueStatusKey =
  | "available"
  | "creating"
  | "unavailable"
  | "unknown";

export type KeyValueBadgeVariant =
  | "default"
  | "secondary"
  | "outline"
  | "destructive";

export interface KeyValueStatus {
  key: KeyValueStatusKey;
  variant: KeyValueBadgeVariant;
}

export interface KeyValueStats {
  total: number;
  available: number;
  creating: number;
}

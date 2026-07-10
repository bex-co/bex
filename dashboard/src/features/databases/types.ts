// bex-native projection of a managed-Postgres row, mapped from bex-api's Render
// dashboard-shaped GraphQL `Database` (backend/internal/postgres/graphql.go).
// The wire type carries Render's `status` enum (available/creating/unavailable)
// and a string `suspended` enum; this view normalizes what the UI renders so
// pages never re-derive Render's encoding (mirrors services' ServiceView).

export interface DatabaseView {
  /** Database name — Render's opaque id; also the detail-page deep-link param. */
  id: string;
  name: string;
  /** Render's databaseStatus enum, verbatim (available/creating/unavailable). */
  status: string;
  /** Plan spelling from the tiers catalog (e.g. "basic-1gb"), or null. */
  plan: string | null;
  /** PostgreSQL major version, or null when the operator default was used. */
  version: string | null;
  /** Provisioned disk size in GB, or null. */
  diskSizeGB: number | null;
  createdAt: string | null;
  /** Whether the external (public) endpoint is enabled. */
  public: boolean;
  /** Render's string suspended enum ("suspended" / "not_suspended"). */
  suspended: string;
}

/** The extra fields the detail query reads beyond the list projection. */
export interface DatabaseDetailView extends DatabaseView {
  /** Normalized (unquoted-identifier) database name. */
  databaseName: string | null;
  /** Owner role, `<db>_user`. */
  databaseUser: string | null;
  highAvailabilityEnabled: boolean;
  /** SNI host for the external endpoint, or null when private. */
  externalHost: string | null;
}

/** The connection strings + password, fetched on demand (never in the list). */
export interface ConnectionInfoView {
  password: string;
  internalConnectionString: string;
  externalConnectionString: string;
  psqlCommand: string;
}

/** One catalog plan as the create dialog's picker consumes it. */
export interface DatabaseInstanceTypeView {
  /** spec.plan spelling — what createDatabase accepts. */
  id: string;
  name: string;
  cpu: string;
  memory: string;
  /** The plan's bundled storage floor, in GB. */
  storageGB: number;
}

/** A resolved status key (i18n label) + the badge variant it renders as. */
export type DatabaseStatusKey =
  | "available"
  | "creating"
  | "unavailable"
  | "unknown";

export type DatabaseBadgeVariant =
  | "default"
  | "secondary"
  | "outline"
  | "destructive";

export interface DatabaseStatus {
  key: DatabaseStatusKey;
  variant: DatabaseBadgeVariant;
}

export interface DatabaseStats {
  total: number;
  available: number;
  creating: number;
}

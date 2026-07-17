// bex-native projection of a managed-Postgres row, mapped from bex-api's Render
// dashboard-shaped GraphQL `Database` (backend/internal/postgres/graphql.go).
// The wire type carries Render's `status` enum (available/creating/unavailable)
// and a string `suspended` enum; this view normalizes what the UI renders so
// pages never re-derive Render's encoding (mirrors services' ServiceView).

export interface DatabaseView {
  /** Immutable Postgres id (dpg-... for new resources); detail-page deep-link param. */
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

/** Host-only connection info for one named read replica (no password). */
export interface ReadReplicaConnectionInfo {
  internalHost: string | null;
  externalHost: string | null;
}

/** One named read replica as returned in the database detail view. */
export interface ReadReplicaView {
  name: string;
  connectionInfo: ReadReplicaConnectionInfo | null;
}

/** The extra fields the detail query reads beyond the list projection. */
export interface DatabaseDetailView extends DatabaseView {
  /** Normalized (unquoted-identifier) database name. */
  databaseName: string | null;
  /** Owner role, `<db>_user`. */
  databaseUser: string | null;
  highAvailabilityEnabled: boolean;
  /** Render's diskAutoscalingEnabled read field. */
  diskAutoscalingEnabled: boolean;
  /** Named read replicas, each with host-only connection info (no password). */
  readReplicas: ReadReplicaView[];
  /** SNI host for the external endpoint, or null when private. */
  externalHost: string | null;
  /** Whether the operator has configured continuous physical backups. */
  backupsEnabled: boolean;
  /** Platform placement name (e.g. "fsn1"); null when BEX_REGION is unset. */
  region: string | null;
}

/** Full connection strings (with password) for one named read replica. */
export interface ReplicaConnectionStrings {
  name: string;
  internalConnectionString: string | null;
  externalConnectionString: string | null;
}

/** The connection strings + password, fetched on demand (never in the list). */
export interface ConnectionInfoView {
  password: string;
  internalConnectionString: string;
  externalConnectionString: string;
  psqlCommand: string;
  /** Per-replica full connection strings (populated when readReplicas exist). */
  readReplicaConnectionStrings: ReplicaConnectionStrings[];
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
  | "upgrading"
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

// --- Insights types (w2/m25) ---

export interface ProcessView {
  pid: number;
  userName: string;
  applicationName: string;
  state: string;
  query?: string | null;
  waitEventType?: string | null;
  waitEvent?: string | null;
  durationSeconds: number;
}

export interface TopQueryView {
  query: string;
  calls: number;
  totalTimeMs: number;
  meanTimeMs: number;
  rows: number;
  sharedHitBlks: number;
  sharedReadBlks: number;
}

export interface DatabaseSizeView {
  name: string;
  sizeBytes: number;
  sizePretty: string;
}

export interface TableSizeView {
  schema: string;
  name: string;
  sizeBytes: number;
  sizePretty: string;
}

export interface DatabaseSizesView {
  database: DatabaseSizeView;
  tables: TableSizeView[];
}

export interface TableScanView {
  schema: string;
  name: string;
  seqScans: number;
  seqScanRows: number;
  indexScans: number;
  indexScanRows: number;
  liveRows: number;
  deadRows: number;
}

export interface ParameterOverrideView {
  name: string;
  setting: string;
  unit?: string | null;
  source: string;
  description?: string | null;
}

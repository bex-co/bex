import type { DatabasesQuery, DatabaseQuery } from "@/graphql/definitions";
import type {
  DatabaseView,
  DatabaseDetailView,
  DatabaseStatus,
  DatabaseStats,
} from "@/features/databases/types";

// Render's databaseStatus enum (backend/internal/postgres/service.go dbStatus):
// "available" once the CNPG cluster is healthy, "creating" while provisioning,
// "unavailable" on failure. The UI keys the badge on this, not on the App-style
// phase — Postgres has its own lifecycle.
export const AVAILABLE = "available";
export const CREATING = "creating";
export const UNAVAILABLE = "unavailable";

/** A single item as it comes off the `databases` query (fields nullable). */
type DatabaseNode = NonNullable<
  NonNullable<DatabasesQuery["databases"]>[number]
>;
type DatabaseDetailNode = NonNullable<DatabaseQuery["database"]>;

/** Map a wire `Database` list node onto the normalized DatabaseView. */
export function toDatabaseView(d: DatabaseNode): DatabaseView {
  return {
    id: d.id ?? "",
    name: d.name ?? d.id ?? "",
    status: d.status ?? "",
    plan: d.plan ?? null,
    version: d.version ?? null,
    diskSizeGB: d.diskSizeGB ?? null,
    createdAt: d.createdAt ?? null,
    public: d.public ?? false,
    suspended: d.suspended ?? "",
  };
}

/** True when a database is suspended (hibernated) — Render's string enum. */
export const SUSPENDED = "suspended";
export function isSuspended(d: { suspended: string }): boolean {
  return d.suspended === SUSPENDED;
}

/**
 * Map the nullable `databases` query result onto the normalized view list —
 * drop null holes, map each node (mirrors services' toServiceViews so the list
 * hook stays thin).
 */
export function toDatabaseViews(
  nodes: DatabasesQuery["databases"] | undefined,
): DatabaseView[] {
  return (nodes ?? [])
    .filter((d): d is DatabaseNode => d != null)
    .map(toDatabaseView);
}

/** Map the single `database(id)` node onto the richer detail view. */
export function toDatabaseDetailView(
  d: DatabaseDetailNode,
): DatabaseDetailView {
  return {
    ...toDatabaseView(d),
    databaseName: d.databaseName ?? null,
    databaseUser: d.databaseUser ?? null,
    highAvailabilityEnabled: d.highAvailabilityEnabled ?? false,
    readReplicas: (d.readReplicas ?? [])
      .filter((r): r is NonNullable<typeof r> => r != null && r.name != null)
      .map((r) => ({
        name: r.name!,
        connectionInfo: r.connectionInfo
          ? {
              internalHost: r.connectionInfo.internalHost ?? null,
              externalHost: r.connectionInfo.externalHost ?? null,
            }
          : null,
      })),
    externalHost: d.externalHost ?? null,
    backupsEnabled: d.backupsEnabled ?? false,
  };
}

// Each Render status maps to a status key + badge variant. Keyed on the
// lower-cased status so a change in the backend's casing can't silently fall
// through to "unknown".
const STATUS_MAP: Record<string, DatabaseStatus> = {
  available: { key: "available", variant: "default" },
  creating: { key: "creating", variant: "outline" },
  upgrading: { key: "upgrading", variant: "secondary" },
  unavailable: { key: "unavailable", variant: "destructive" },
};

/** Resolve a database's display status from Render's status enum. */
export function deriveStatus(d: { status: string }): DatabaseStatus {
  const status = STATUS_MAP[d.status.toLowerCase()];
  if (status) return status;
  return { key: "unknown", variant: "outline" };
}

/** True while the database is still converging (used to poll the list live). */
export function isConverging(d: { status: string }): boolean {
  const key = deriveStatus(d).key;
  return key === "creating" || key === "upgrading";
}

/** Stat-tile counts computed from the live list (total / available / creating). */
export function computeStats(databases: DatabaseView[]): DatabaseStats {
  return {
    total: databases.length,
    available: databases.filter((d) => deriveStatus(d).key === "available")
      .length,
    creating: databases.filter((d) => deriveStatus(d).key === "creating")
      .length,
  };
}

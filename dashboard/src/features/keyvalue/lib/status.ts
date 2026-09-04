import type { KeyValuesQuery, KeyValueQuery } from "@/graphql/definitions";
import { isSuspended } from "@/features/services/lib/status";
import type {
  KeyValueView,
  KeyValueStatus,
  KeyValueStats,
} from "@/features/keyvalue/types";

// Render's keyValueStatus enum (backend/internal/keyvalue/service.go kvStatus):
// "available" once the Valkey StatefulSet is ready, "creating" while
// provisioning, "unavailable" on failure.
export const AVAILABLE = "available";
export const CREATING = "creating";
export const UNAVAILABLE = "unavailable";

/** A single item as it comes off the `keyValues` query (fields nullable). */
type KeyValueNode = NonNullable<
  NonNullable<KeyValuesQuery["keyValues"]>[number]
>;
type KeyValueDetailNode = NonNullable<KeyValueQuery["keyValue"]>;

/** Map a wire `KeyValue` node onto the normalized KeyValueView. Shared by the
 * list and detail queries — there's no separate detail-only field set. */
export function toKeyValueView(
  d: KeyValueNode | KeyValueDetailNode,
): KeyValueView {
  return {
    id: d.id ?? "",
    name: d.name ?? d.id ?? "",
    status: d.status ?? "",
    plan: d.plan ?? null,
    version: d.version ?? null,
    createdAt: d.createdAt ?? null,
    updatedAt: "updatedAt" in d ? (d.updatedAt ?? null) : null,
    // bex-api's contract for a private store is empty-string, not null
    // (w6/052) — normalize to null so truthiness checks and row-omission
    // work the same for both encodings.
    externalHost: d.externalHost || null,
    public: d.public ?? false,
    suspended: isSuspended(d.suspended ?? null),
    region: "region" in d ? d.region || null : null,
  };
}

/**
 * Map the nullable `keyValues` query result onto the normalized view list —
 * drop null holes, map each node (mirrors databases' toDatabaseViews).
 */
export function toKeyValueViews(
  nodes: KeyValuesQuery["keyValues"] | undefined,
): KeyValueView[] {
  return (nodes ?? [])
    .filter((d): d is KeyValueNode => d != null)
    .map(toKeyValueView);
}

// Each Render status maps to a status key + badge variant. Keyed on the
// lower-cased status so a change in the backend's casing can't silently fall
// through to "unknown".
const STATUS_MAP: Record<string, KeyValueStatus> = {
  available: { key: "available", variant: "default" },
  creating: { key: "creating", variant: "outline" },
  unavailable: { key: "unavailable", variant: "destructive" },
};

function fromStatus(status: string): KeyValueStatus {
  return (
    STATUS_MAP[status.toLowerCase()] ?? { key: "unknown", variant: "outline" }
  );
}

/**
 * Resolve a Key Value store's display status. Suspension wins over the status
 * enum — a suspended store still reports status "available" (Render keeps
 * `status` and `suspended` as separate fields), but "suspended" is the state
 * the user asked for and acts on, so it's what the badge shows (mirrors
 * services' and databases' deriveStatus).
 */
export function deriveStatus(d: {
  status: string;
  suspended: boolean;
}): KeyValueStatus {
  if (d.suspended) return { key: "suspended", variant: "secondary" };
  return fromStatus(d.status);
}

/** True while the store is still converging (used to poll the list/detail live). */
export function isConverging(d: { status: string }): boolean {
  return fromStatus(d.status).key === "creating";
}

/** Stat-tile counts computed from the live list (total / available / creating). */
export function computeStats(stores: KeyValueView[]): KeyValueStats {
  return {
    total: stores.length,
    available: stores.filter((d) => deriveStatus(d).key === "available").length,
    creating: stores.filter((d) => deriveStatus(d).key === "creating").length,
  };
}

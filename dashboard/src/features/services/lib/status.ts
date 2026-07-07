import type { ServicesQuery } from "@/graphql/definitions";
import type {
  ServiceView,
  ServiceStatus,
  ServiceStats,
} from "@/features/services/types";

// Render's `suspended` is a string enum, NOT a boolean: "suspended" means the
// App is parked, "not_suspended" means it's live (backend/internal/api/render.go).
export const SUSPENDED = "suspended";
export const NOT_SUSPENDED = "not_suspended";

/** A single service item as it comes off the `services` query (fields nullable). */
type ServiceNode = NonNullable<NonNullable<ServicesQuery["services"]>[number]>;

/** True when Render's string enum marks the App as suspended. */
export function isSuspended(suspended: string | null): boolean {
  return suspended === SUSPENDED;
}

/** Map a wire `Service` onto the normalized ServiceView the UI renders. */
export function toServiceView(s: ServiceNode): ServiceView {
  return {
    id: s.id ?? "",
    name: s.name ?? s.id ?? "",
    suspended: isSuspended(s.suspended),
    phase: s.phase ?? "",
    url: s.url ?? null,
    createdAt: s.createdAt ?? null,
    replicas: s.replicas ?? null,
    revision: s.revision ?? null,
  };
}

// Each operator phase maps to a status key + badge variant. Keyed on the
// lower-cased phase so a change in the operator's casing can't silently fall
// through to "unknown".
const PHASE_STATUS: Record<string, ServiceStatus> = {
  running: { key: "running", variant: "default" },
  deploying: { key: "deploying", variant: "outline" },
  building: { key: "building", variant: "outline" },
  pending: { key: "pending", variant: "outline" },
  hibernated: { key: "hibernated", variant: "secondary" },
  failed: { key: "failed", variant: "destructive" },
};

/**
 * Resolve a service's display status. Suspension wins over phase — a suspended
 * App scales to 0 and reports phase Hibernated, but "suspended" is the state the
 * operator asked for and the one the user acts on, so it's what the badge shows.
 */
export function deriveStatus(s: ServiceView): ServiceStatus {
  if (s.suspended) return { key: "suspended", variant: "secondary" };
  const status = PHASE_STATUS[s.phase.toLowerCase()];
  if (status) return status;
  return { key: "unknown", variant: "outline" };
}

/** Stat-tile counts computed from the live list (total / running / suspended). */
export function computeStats(services: ServiceView[]): ServiceStats {
  return {
    total: services.length,
    running: services.filter((s) => deriveStatus(s).key === "running").length,
    suspended: services.filter((s) => s.suspended).length,
  };
}

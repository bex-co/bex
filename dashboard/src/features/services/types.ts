// bex-native projection of a Service row, mapped from bex-api's Render-shaped
// GraphQL `Service` (backend/internal/api/graphql.go). The wire type carries a
// string `suspended` enum ("suspended"/"not_suspended") and a capitalized
// `phase` (Running/Hibernated/Failed/…); this view normalizes both so the UI
// never re-derives Render's encoding.

export interface ServiceView {
  /** App name — Render's opaque service id; also the metrics deep-link param. */
  id: string;
  name: string;
  /** Derived from Render's string `suspended` enum, not a raw boolean field. */
  suspended: boolean;
  /** Operator phase, verbatim (Pending/Building/Deploying/Running/Hibernated/Failed). */
  phase: string;
  /** Live URL, or null when the App has none yet. */
  url: string | null;
  createdAt: string | null;
  replicas: number | null;
  revision: string | null;
}

/** The lifecycle verbs the row exposes, named after bex-api's Render mutations. */
export type LifecycleAction = "suspend" | "resume" | "restart";

/** A resolved status key (i18n label) + the badge variant it renders as. */
export type ServiceStatusKey =
  | "running"
  | "suspended"
  | "hibernated"
  | "pending"
  | "building"
  | "deploying"
  | "failed"
  | "unknown";

export type ServiceBadgeVariant =
  | "default"
  | "secondary"
  | "outline"
  | "destructive";

export interface ServiceStatus {
  key: ServiceStatusKey;
  variant: ServiceBadgeVariant;
}

export interface ServiceStats {
  total: number;
  running: number;
  suspended: number;
}

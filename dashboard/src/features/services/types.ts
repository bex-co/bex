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
  /** Render's plan spelling (e.g. "pro_plus"), or null for an untiered App. */
  plan: string | null;
  /**
   * Free-tier auto-sleep window in seconds (bex extension, `spec.idleTTLSeconds`);
   * 0 means the controller default. null when the wire result didn't select it
   * (the list query omits it — only the detail `server` query fetches it for the
   * Settings tab).
   */
  idleTTLSeconds: number | null;
}

/** The lifecycle verbs the row exposes, named after bex-api's Render mutations. */
export type LifecycleAction = "suspend" | "resume" | "restart";

/**
 * One env-var key on the Environment tab (Render dashboard shape: the list is
 * keys only; a value is fetched per key on "Show secret"). `id` is bex-api's
 * per-var id, which equals the key.
 */
export interface EnvVarKey {
  id: string;
  key: string;
}

/**
 * One custom domain on a service's Settings tab (Render dashboard shape). bex-api
 * uses the hostname as the opaque id (id === name), so this view carries just the
 * name. `verified` and `active` are derived from bex-api's string status fields
 * (verificationStatus / serverStatus) so the UI never re-encodes them:
 * - `verified` — the TLS certificate has been issued for the host (Render's
 *   "Verified Status" column).
 * - `active` — the certificate is issued and the service isn't suspended, so it's
 *   actively serving the host (Render's "Certificate Status" column).
 */
export interface CustomDomainView {
  /** The FQDN — also bex-api's opaque id for the domain. */
  name: string;
  verified: boolean;
  active: boolean;
}

/** A resolved status key (i18n label) + the badge variant it renders as. */
export type ServiceStatusKey =
  | "running"
  | "suspended"
  // "sleeping" = auto-hibernated free-tier App (phase Hibernated && not
  // manually suspended). Distinct from "suspended" so the UI can explain
  // "wakes on the next request" — a deliberate bex divergence from Render,
  // which keeps spun-down free services showing as live.
  | "sleeping"
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

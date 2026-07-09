// bex-native projection of a Service row, mapped from bex-api's Render-shaped
// GraphQL `Service` (backend/internal/api/graphql.go). The wire type carries a
// string `suspended` enum ("suspended"/"not_suspended") and a capitalized
// `phase` (Running/Hibernated/Failed/…); this view normalizes both so the UI
// never re-derives Render's encoding.

export interface ServiceView {
  /** App name — Render's opaque service id; also the metrics deep-link param. */
  id: string;
  name: string;
  /**
   * Render serviceType — web_service | private_service | background_worker |
   * cron_job. Empty spec.type reads back as web_service from bex-api.
   */
  type: string;
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
  /**
   * Cron schedule (5-field crontab), only set for a `cron_job`; null otherwise.
   * Only the detail `server` query selects it.
   */
  schedule: string | null;
  /**
   * Overrides a `cron_job`'s default entrypoint (Render's cron "Command" field);
   * null for other types, or when the image's own command runs unmodified. Only
   * the detail `server` query selects it.
   */
  command: string | null;
  /**
   * A `cron_job`'s recent run history (newest first), only selected by the detail
   * `server` query. Empty for other types / when not selected.
   */
  runs: CronRunView[];
}

/** One execution of a `cron_job` — the Render cron-run shape bex-api projects. */
export interface CronRunView {
  /** Kubernetes Job name backing the run — its stable id in the list. */
  name: string;
  /** RFC3339 start time, or null if it hasn't started. */
  startedAt: string | null;
  /** RFC3339 completion/failure time, or null while running. */
  finishedAt: string | null;
  /** Run outcome — Running | Succeeded | Failed. */
  status: string;
}

/** The lifecycle verbs the row exposes, named after bex-api's Render mutations. */
export type LifecycleAction = "suspend" | "resume" | "restart";

/** A resolved service-type key (i18n label) + the badge variant it renders as. */
export type ServiceTypeKey = "web" | "private" | "worker" | "cron" | "unknown";

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
 * One secret file on the Environment tab (per-service, Render dashboard shape:
 * the list is names only; a file's content is fetched on demand on "Show").
 * `id` is bex-api's per-file id, which equals the file name.
 */
export interface SecretFileName {
  id: string;
  name: string;
}

/**
 * One environment group (a reusable bundle of env vars + secret files that can be
 * linked to multiple services). Mapped from bex-api's nullable `EnvGroup` wire
 * shape so the UI never re-derives the encoding. `serviceLinks` holds the service
 * ids this group is attached to; `envVarKeys`/`secretFileNames` are read-only
 * previews of the group's contents (keys/names only, no values).
 */
export interface EnvGroupView {
  id: string;
  name: string;
  serviceLinks: string[];
  envVarKeys: string[];
  secretFileNames: string[];
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
/** The DNS record the tenant creates to point a custom domain at the service. */
export interface DnsRecordView {
  /** "CNAME" (subdomain) or "ALIAS" (apex). */
  type: string;
  /** The record host to create: the subdomain label(s), or "@" for apex. */
  name: string;
  /** The target the record points to: the app's platform host <app>.<base-domain>. */
  value: string;
}

export interface CustomDomainView {
  /** The FQDN — also bex-api's opaque id for the domain. */
  name: string;
  /** "apex" or "subdomain" — drives the apex vs subdomain DNS guidance. */
  domainType: string;
  verified: boolean;
  active: boolean;
  /** The DNS record to create; null if the backend couldn't derive the target. */
  dnsRecord: DnsRecordView | null;
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

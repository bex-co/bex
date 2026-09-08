// bex-native projection of a Service row, mapped from bex-api's Render-shaped
// GraphQL `Service` (backend/internal/api/graphql.go). The wire type carries a
// string `suspended` enum ("suspended"/"not_suspended") and a capitalized
// `phase` (Running/Hibernated/Failed/…); this view normalizes both so the UI
// never re-derives Render's encoding.

export interface ServiceView {
  /** App name — Render's opaque service id; also the metrics deep-link param. */
  id: string;
  /** Human-facing label: displayName when set, otherwise the immutable App name. */
  name: string;
  /**
   * The globally-unique platform-host segment (Render's "slug" field;
   * `spec.subdomain`, minted w4/m19) — distinct from `name`, which is only
   * workspace-unique. Null when not selected (only the detail `server` query
   * fetches it).
   */
  slug: string | null;
  /** Raw mutable label from spec.displayName; null/empty means fall back to id. */
  displayName?: string | null;
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
  /**
   * Why an exposed service has NO public address (w7/m79): the platform
   * subdomain is unavailable on this installation and no custom domain is
   * attached. Null when the service is routed, or is not the kind that carries
   * a public URL (a worker, a cron job, or an owner who switched their own
   * platform subdomain off) — so the header can render it unconditionally.
   *
   * The text is the operator's own diagnosis, passed through like a deploy's
   * failureReason rather than reconstructed here: only the operator knows the
   * base domain actually in effect.
   */
  publicRoutingNotice?: string | null;
  /**
   * Private-network address sibling services connect to — "<slug>:<port>",
   * scheme-less (Render's Connect → Internal string; ADR041 D4, w9/m58).
   * Web and private services only; null otherwise or when not selected
   * (only the detail `server` query fetches it). A private service's header
   * shows it as its Service Address instead of a public-URL link.
   */
  internalAddress: string | null;
  createdAt: string | null;
  /** Authoritative last resource mutation time; absent on legacy list fixtures. */
  updatedAt?: string | null;
  /** Explicit installation placement (`BEX_REGION`), never browser-inferred. */
  region?: string | null;
  /** Copy-ready Render-compatible raw OpenSSH target, or null when unavailable. */
  sshAddress: string | null;
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
   * When a `cron_job` last completed successfully (RFC3339, Render's
   * `cronJobDetails.lastSuccessfulRunAt`); null when it has never succeeded or
   * for other types. Only the detail `server` query selects it.
   */
  lastSuccessfulRunAt?: string | null;
  /**
   * A `cron_job`'s next scheduled fire time (RFC3339 UTC, a bex extension
   * computed from the schedule); null for a suspended/non-cron service or an
   * unparseable schedule. Only the detail `server` query selects it.
   */
  nextRunAt?: string | null;
  /**
   * A `cron_job`'s recent run history (newest first), only selected by the detail
   * `server` query. Empty for other types / when not selected.
   */
  runs: CronRunView[];
  /**
   * Build-from-git source (`spec.repo`/`spec.branch`); null for an image-backed
   * App. Only the detail `server` query selects these (w1/m18, w5/m13).
   */
  repo: string | null;
  branch: string | null;
  /**
   * Configured prebuilt image (`spec.image`), empty/null for a repo-backed App.
   * Optional so fixtures that predate the Source card (w5/m76) still satisfy the
   * type; the `server` detail query and `status.ts` always populate it.
   */
  imagePath?: string | null;
  /**
   * Subdirectory of `repo` this App builds from (`spec.rootDir`, Render's Root
   * Directory setting, w1/m18); null when unset (builds from the repo root).
   * Only the detail `server` query selects it.
   */
  rootDir: string | null;
  /** Render runtime selected for this service (docker or a native language). */
  runtime: string | null;
  /** Internal build strategy, used to recognize legacy Dockerfile-built Apps. */
  builder: string | null;
  /**
   * Render's Build Command (`spec.buildCommand`, w7/m41): the shell command that
   * produces build artifacts (e.g., npm run build). null/empty means the runtime
   * default. Only selected by the detail `server` query.
   */
  buildCommand: string | null;
  /** Native Start Command or Docker Command override. */
  startCommand: string | null;
  /** Dockerfile path relative to rootDir; empty means Dockerfile. */
  dockerfilePath: string | null;
  /**
   * Stored private-registry credential bound to a prebuilt image or repository
   * Docker build. Empty/null means no explicit credential. Only selected by the
   * detail `server` query.
   */
  registryCredentialId: string | null;
  /**
   * Render's Build Filters (`spec.buildFilter`, w1/m34): repository-root-relative
   * glob patterns gating git-push auto-deploys. null when unset (every matching
   * push deploys) or not selected. Only the detail `server` query selects it.
   */
  buildFilter: BuildFilterView | null;
  /**
   * Whether a signed git push redeploys this App (`spec.autoDeploy`, w2/m9);
   * null when not selected (list query) or for an image-backed App.
   */
  autoDeploy: boolean | null;
  /**
   * How a push to the tracked branch can actually REACH bex for this specific
   * repo (w6/m99): "github_app" | "manual_webhook" | "none" | "unknown" —
   * what the autoDeploy on/off setting above cannot express. null on the list
   * query, which does not select it, and on an older API.
   */
  pushDeliveryMethod?: string | null;
  /**
   * Deploy-failure notification override (`spec.notifyOnFail`, w4/m21, Render's
   * exact field name/enum — docs/render-artifacts/notify-on-fail.md):
   * "default" | "notify" | "ignore". null when not selected (list query);
   * bex-api reports empty as "default", never a bare empty string.
   */
  notifyOnFail: string | null;
  /** Render's service notification override: default | none | failure | all. */
  notificationsToSend: string | null;
  /**
   * HTTP path the health probes (readiness + liveness, w7/m81) GET
   * (`spec.healthCheckPath`, w1/m23/t001); null/empty selects the TCP default
   * — the platform checks only that the process is listening (w7/m80). Only
   * applies to web_service and private_service; null when not selected (list query).
   */
  healthCheckPath: string | null;
  /**
   * Seconds Kubernetes waits after SIGTERM before SIGKILL (1-300; default 30).
   * Only web/private/background-worker services expose this setting.
   */
  maxShutdownDelaySeconds: number | null;
  /**
   * Render's Pre-Deploy Command (`spec.preDeployCommand`, w1/m33): a command run
   * to completion against the new image before it serves traffic (typically a DB
   * migration). null/empty means no pre-deploy step; null when not selected (list
   * query). Read/written by the Settings → Build & Deploy section.
   */
  preDeployCommand: string | null;
  /**
   * Render's renderSubdomainPolicy (`spec.subdomainPolicy`, w7/m31):
   * "enabled" (default, platform subdomain active) | "disabled" (platform host
   * dropped; only custom domains serve). null when not selected (list query).
   */
  renderSubdomainPolicy: string | null;
  /**
   * Built output directory a `static_site` serves (`spec.publishPath`, Render's
   * Publish Directory); null for other types / when not selected. Only the detail
   * `server` query selects it.
   */
  publishPath: string | null;
  /**
   * A `static_site`'s ordered redirect/rewrite rules (`spec.routes`, Render's
   * /routes); empty for other types / when not selected.
   */
  routes: StaticRouteView[];
  /**
   * A `static_site`'s custom response-header rules (`spec.headers`, Render's
   * /headers); empty for other types / when not selected.
   */
  headers: StaticHeaderView[];
  /**
   * Render's inbound CIDR allowlist for `web_service` and `static_site`
   * (`spec.ipAllowList`, w7/m32). Empty/null means open to all source IPs.
   * null when not selected (list query omits it); only the detail `server`
   * query fetches it for the Networking section.
   */
  ipAllowList: Array<string | null> | null;
  /** Description-preserving form used by the Settings Networking editor. */
  ipAllowListEntries: Array<{
    cidrBlock: string;
    description: string | null;
  } | null> | null;
  /**
   * Shared egress IPs (`Service.outboundIps`, w8/010). null when not selected
   * (list query); detail `server` query returns `{type, ips}` (always shared).
   */
  outboundIps: { type: string; ips: string[] } | null;
  /**
   * Render's maintenanceMode object (`spec.maintenanceMode`, w1/m37): takes a
   * web_service offline behind an interstitial page without suspending it.
   * null when not selected (list query); the detail `server` query always
   * selects a concrete object (bex-api reports the zero value
   * `{enabled: false, uri: ""}` even when never configured).
   */
  maintenanceMode: MaintenanceModeView | null;
}

/**
 * Render's maintenanceMode object (`spec.maintenanceMode`, w1/m37):
 * `enabled` takes the service offline behind an interstitial page; `uri` is
 * an optional absolute http(s) URL to a custom page (empty uses bex's
 * default page).
 */
export interface MaintenanceModeView {
  enabled: boolean;
  uri: string;
}

export interface MaintenanceModeView {
  enabled: boolean;
  /** Empty selects the platform's default 503 maintenance page. */
  uri: string;
}

/**
 * Render's Build Filters object (`spec.buildFilter`): the glob patterns gating
 * git-push auto-deploys. `paths` are include globs (empty = every path);
 * `ignoredPaths` are exclude globs (ignored wins over included).
 */
export interface BuildFilterView {
  paths: string[];
  ignoredPaths: string[];
}

/** One redirect/rewrite rule for a static_site (Render's route shape). */
export interface StaticRouteView {
  /** "redirect" (301 to destination) or "rewrite" (serve destination, 200). */
  type: string;
  /** Request path pattern to match, e.g. "/old" or "/app/*". */
  source: string;
  /** Target path, e.g. "/new" or "/index.html". */
  destination: string;
}

/** One custom response-header rule for a static_site (Render's header shape). */
export interface StaticHeaderView {
  /** Request path pattern the header applies to, e.g. "/*". */
  path: string;
  name: string;
  value: string;
}

/** One execution of a `cron_job` — the Render cron-run shape bex-api projects. */
export interface CronRunView {
  /** Stable, opaque crr- id. */
  id: string;
  /** RFC3339 start time, or null if it hasn't started. */
  startedAt: string | null;
  /** RFC3339 completion/failure time, or null while running. */
  finishedAt: string | null;
  /** Render run outcome — pending | successful | unsuccessful | canceled. */
  status: string;
}

/** The lifecycle verbs the row exposes, named after bex-api's Render mutations. */
export type LifecycleAction = "suspend" | "resume" | "restart";

/** A resolved service-type key (i18n label) + the badge variant it renders as. */
export type ServiceTypeKey =
  | "web"
  | "private"
  | "worker"
  | "cron"
  | "static"
  | "unknown";

/**
 * One env-var key on a sensitive-config list. Values are fetched individually
 * only when the user asks to reveal them; the wire id equals the key.
 */
export interface EnvVarKey {
  id: string;
  key: string;
}

/**
 * One secret-file name on a sensitive-config list. Contents are fetched
 * individually only when the user asks to reveal them; the wire id equals the
 * name.
 */
export interface SecretFileName {
  id: string;
  name: string;
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
  /** Durable DNS-TXT ownership admission; false means the host is not routed. */
  ownershipVerified: boolean;
  /** TLS certificate issued after ownership admission. */
  verified: boolean;
  active: boolean;
  /** Canonical host when this auto-paired sibling redirects; null when served directly. */
  redirectForName: string | null;
  /** The DNS record to create; null if the backend couldn't derive the target. */
  dnsRecord: DnsRecordView | null;
  /** Exact TXT proof while ownership is pending; null after atomic promotion. */
  ownershipDnsRecord: DnsRecordView | null;
}

/**
 * Finds the other half of an active auto-pair redirect. The backend-projected
 * redirectForName is authoritative: two explicitly claimed www/apex hosts are
 * both served directly and therefore are not presented as an auto-pair.
 */
export function pairedSibling(
  domain: CustomDomainView,
  domains: CustomDomainView[],
): CustomDomainView | null {
  if (domain.redirectForName) {
    return domains.find((d) => d.name === domain.redirectForName) ?? null;
  }
  return domains.find((d) => d.redirectForName === domain.name) ?? null;
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
  // "canceled" = the user stopped the release that was rolling and no earlier
  // one ever succeeded. Deliberately not folded into "failed" — see PHASE_STATUS
  // in lib/status.ts (w6/m52).
  | "canceled"
  // "deleting" = the service's deletion has been accepted and its finalizer is
  // tearing it down. By-id reads return not-found the instant deletion is
  // accepted (w3/m81), so the dashboard normally redirects rather than showing
  // this — it exists so a service ever observed mid-teardown reads honestly
  // (a muted "Deleting" badge, no dead URL) instead of the generic "Unknown".
  | "deleting"
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

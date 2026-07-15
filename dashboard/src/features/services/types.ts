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
  /**
   * Build-from-git source (`spec.repo`/`spec.branch`); null for an image-backed
   * App. Only the detail `server` query selects these (w1/m18, w5/m13).
   */
  repo: string | null;
  branch: string | null;
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
  /** Native Start Command or Docker Command override. */
  startCommand: string | null;
  /** Dockerfile path relative to rootDir; empty means Dockerfile. */
  dockerfilePath: string | null;
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
   * Deploy-failure notification override (`spec.notifyOnFail`, w4/m21, Render's
   * exact field name/enum — docs/render-artifacts/notify-on-fail.md):
   * "default" | "notify" | "ignore". null when not selected (list query);
   * bex-api reports empty as "default", never a bare empty string.
   */
  notifyOnFail: string | null;
  /**
   * HTTP path the ReadinessProbe polls before routing traffic (`spec.healthCheckPath`,
   * w1/m23/t001); null/empty means the platform default "/". Only applies to
   * web_service and private_service; null when not selected (list query).
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
  /** Legacy nested-service alias; first-class reads set this to id. */
  name: string;
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
  verified: boolean;
  active: boolean;
  /** The DNS record to create; null if the backend couldn't derive the target. */
  dnsRecord: DnsRecordView | null;
}

/**
 * Finds domain's www<->apex auto-pairing sibling (w6/m23) within the same
 * domains list, if present — a display-only mirror of the backend's wwwSibling
 * (lego/backend/internal/apps/domains.go), built on the backend-computed
 * `domainType` (the real public-suffix list) rather than reimplementing PSL
 * client-side: an apex's sibling is "www."+name; a "www."+X domain's sibling is
 * X, but only when X is present AND itself marked "apex" (guards against
 * mis-pairing an unrelated "www.foo" whose "foo" happens to be someone else's
 * subdomain, not X's own registrable domain).
 */
export function pairedSibling(
  domain: CustomDomainView,
  domains: CustomDomainView[],
): CustomDomainView | null {
  if (domain.domainType === "apex") {
    return domains.find((d) => d.name === `www.${domain.name}`) ?? null;
  }
  if (domain.name.startsWith("www.")) {
    const apexName = domain.name.slice(4);
    return (
      domains.find((d) => d.name === apexName && d.domainType === "apex") ??
      null
    );
  }
  return null;
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

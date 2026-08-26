import { servesHttp } from "@/features/services/lib/service-type";
import type { ServicesQuery, ServerQuery } from "@/graphql/definitions";
import type {
  ServiceView,
  ServiceStatus,
  ServiceStats,
  CronRunView,
  StaticRouteView,
  StaticHeaderView,
  BuildFilterView,
  MaintenanceModeView,
} from "@/features/services/types";

// Render's `suspended` is a string enum, NOT a boolean: "suspended" means the
// App is parked, "not_suspended" means it's live (backend/internal/api/render.go).
export const SUSPENDED = "suspended";
export const NOT_SUSPENDED = "not_suspended";

/** A single service item as it comes off the `services` query (fields nullable). */
type ServiceNode = NonNullable<NonNullable<ServicesQuery["services"]>[number]>;
/** The single-service node from the detail `server` query — carries schedule/runs. */
type ServerNode = NonNullable<ServerQuery["server"]>;

/** True when Render's string enum marks the App as suspended. */
export function isSuspended(suspended: string | null): boolean {
  return suspended === SUSPENDED;
}

/**
 * Map a wire `Service` onto the normalized ServiceView the UI renders. Accepts
 * either the list node or the detail (`server`) node — only the latter selects
 * `schedule`/`runs`, so those are read defensively (list rows leave them empty).
 */
export function toServiceView(s: ServiceNode | ServerNode): ServiceView {
  const immutableName = s.name ?? s.id ?? "";
  const displayName = s.displayName?.trim() || null;
  return {
    id: s.id ?? "",
    name: displayName ?? immutableName,
    slug: "slug" in s ? (s.slug ?? null) : null,
    displayName,
    type: s.type ?? "web_service",
    suspended: isSuspended(s.suspended),
    phase: s.phase ?? "",
    url: s.url ?? null,
    internalAddress: "internalAddress" in s ? s.internalAddress || null : null,
    createdAt: s.createdAt ?? null,
    updatedAt: "updatedAt" in s ? (s.updatedAt ?? null) : null,
    region: "region" in s ? s.region || null : null,
    sshAddress: "sshAddress" in s ? (s.sshAddress ?? null) : null,
    replicas: s.replicas ?? null,
    revision: s.revision ?? null,
    plan: s.plan ?? null,
    idleTTLSeconds: s.idleTTLSeconds ?? null,
    schedule: "schedule" in s ? (s.schedule ?? null) : null,
    command: "command" in s ? (s.command ?? null) : null,
    lastSuccessfulRunAt:
      "lastSuccessfulRunAt" in s ? (s.lastSuccessfulRunAt ?? null) : null,
    nextRunAt: "nextRunAt" in s ? (s.nextRunAt ?? null) : null,
    runs: "runs" in s ? toCronRuns(s.runs) : [],
    repo: "repo" in s ? (s.repo ?? null) : null,
    branch: "branch" in s ? (s.branch ?? null) : null,
    imagePath: "imagePath" in s ? (s.imagePath ?? null) : null,
    rootDir: "rootDir" in s ? (s.rootDir ?? null) : null,
    runtime: "runtime" in s ? (s.runtime ?? null) : null,
    builder: "builder" in s ? (s.builder ?? null) : null,
    buildCommand: "buildCommand" in s ? (s.buildCommand ?? null) : null,
    startCommand: "startCommand" in s ? (s.startCommand ?? null) : null,
    dockerfilePath: "dockerfilePath" in s ? (s.dockerfilePath ?? null) : null,
    registryCredentialId:
      "registryCredentialId" in s ? (s.registryCredentialId ?? null) : null,
    buildFilter: "buildFilter" in s ? toBuildFilter(s.buildFilter) : null,
    autoDeploy: "autoDeploy" in s ? (s.autoDeploy ?? null) : null,
    pushDeliveryMethod:
      "pushDeliveryMethod" in s ? (s.pushDeliveryMethod ?? null) : null,
    notifyOnFail: "notifyOnFail" in s ? (s.notifyOnFail ?? null) : null,
    notificationsToSend:
      "notificationsToSend" in s ? (s.notificationsToSend ?? null) : null,
    renderSubdomainPolicy:
      "renderSubdomainPolicy" in s ? (s.renderSubdomainPolicy ?? null) : null,
    healthCheckPath:
      "healthCheckPath" in s ? (s.healthCheckPath ?? null) : null,
    maxShutdownDelaySeconds:
      "maxShutdownDelaySeconds" in s
        ? (s.maxShutdownDelaySeconds ?? null)
        : null,
    preDeployCommand:
      "preDeployCommand" in s ? (s.preDeployCommand ?? null) : null,
    publishPath: "publishPath" in s ? (s.publishPath ?? null) : null,
    ipAllowList: "ipAllowList" in s ? (s.ipAllowList ?? null) : null,
    ipAllowListEntries:
      "ipAllowListEntries" in s ? (s.ipAllowListEntries ?? null) : null,
    routes: "routes" in s ? toStaticRoutes(s.routes) : [],
    headers: "headers" in s ? toStaticHeaders(s.headers) : [],
    maintenanceMode:
      "maintenanceMode" in s ? toMaintenanceMode(s.maintenanceMode) : null,
  };
}

/**
 * Project the detail query's `maintenanceMode` object onto MaintenanceModeView.
 * bex-api always reports a concrete object (never null); this stays nil-safe
 * so a still-nullable wire field (schema drift, a stale cached response) falls
 * back to disabled rather than throwing.
 */
function toMaintenanceMode(
  m: ServerNode["maintenanceMode"],
): MaintenanceModeView {
  return { enabled: m?.enabled ?? false, uri: m?.uri ?? "" };
}

/** Project the detail query's nullable `routes` array onto StaticRouteView[]. */
function toStaticRoutes(routes: ServerNode["routes"]): StaticRouteView[] {
  return (routes ?? [])
    .filter((r): r is NonNullable<typeof r> => r != null)
    .map((r) => ({
      type: r.type ?? "",
      source: r.source ?? "",
      destination: r.destination ?? "",
    }));
}

/** Project the detail query's nullable `headers` array onto StaticHeaderView[]. */
function toStaticHeaders(headers: ServerNode["headers"]): StaticHeaderView[] {
  return (headers ?? [])
    .filter((h): h is NonNullable<typeof h> => h != null)
    .map((h) => ({
      path: h.path ?? "",
      name: h.name ?? "",
      value: h.value ?? "",
    }));
}

/**
 * Project the detail query's nullable `buildFilter` object onto BuildFilterView.
 * null (unset) stays null; a present object's arrays are read defensively so a
 * filter carrying only one list still yields a well-formed view.
 */
function toBuildFilter(
  buildFilter: ServerNode["buildFilter"],
): BuildFilterView | null {
  if (!buildFilter) return null;
  return {
    paths: (buildFilter.paths ?? []).filter((p): p is string => p != null),
    ignoredPaths: (buildFilter.ignoredPaths ?? []).filter(
      (p): p is string => p != null,
    ),
  };
}

/** Project the detail query's nullable `runs` array onto CronRunView[]. */
function toCronRuns(runs: ServerNode["runs"]): CronRunView[] {
  return (runs ?? [])
    .filter((r): r is NonNullable<typeof r> => r != null)
    .map((r) => ({
      id: r.id ?? r.name ?? "",
      startedAt: r.startedAt ?? null,
      finishedAt: r.finishedAt ?? null,
      status: r.status ?? "",
    }));
}

/**
 * Map the nullable `services` query result onto the normalized view list —
 * the list-level projection (drop null holes, map each node), sealed here next
 * to `toServiceView` so `useServices` stays thin (mirrors logs' `toLogLines`).
 */
export function toServiceViews(
  nodes: ServicesQuery["services"] | undefined,
): ServiceView[] {
  return (nodes ?? [])
    .filter((s): s is ServiceNode => s != null)
    .map(toServiceView);
}

const PENDING: ServiceStatus = { key: "pending", variant: "outline" };

// Each operator phase maps to a status key + badge variant. Keyed on the
// lower-cased phase so a change in the operator's casing can't silently fall
// through to "unknown".
const PHASE_STATUS: Record<string, ServiceStatus> = {
  running: { key: "running", variant: "default" },
  deploying: { key: "deploying", variant: "outline" },
  building: { key: "building", variant: "outline" },
  pending: PENDING,
  // Phase Hibernated reached here means the App auto-slept (idle past its TTL):
  // manual suspend is caught earlier by deriveStatus (suspension wins), so a
  // Hibernated App that is NOT suspended is a free-tier sleeper — shown as
  // "sleeping" with the wake-on-request hint, a bex extension over Render.
  hibernated: { key: "sleeping", variant: "secondary" },
  // Phase Canceled means the user stopped the release that was rolling before
  // any release had ever succeeded — so there is nothing running, but nothing
  // failed either. Non-destructive on purpose (w6/m52): the red "Failed" badge
  // it used to share told the user their own Cancel had broken something.
  canceled: { key: "canceled", variant: "secondary" },
  failed: { key: "failed", variant: "destructive" },
};

/**
 * Resolve a service's display status. Suspension wins over phase — a suspended
 * App scales to 0 and reports phase Hibernated, but "suspended" is the state the
 * operator asked for and the one the user acts on, so it's what the badge shows.
 */
export function deriveStatus(s: ServiceView): ServiceStatus {
  if (s.suspended) return { key: "suspended", variant: "secondary" };
  const phase = s.phase.toLowerCase();
  // A type with no HTTP port never auto-sleeps — no request could reach it to
  // wake it — so Hibernated there is the transient window of a resume, not a
  // sleeper. Reporting "wakes on the next request" would be a promise nothing
  // can keep.
  if (phase === "hibernated" && !servesHttp(s.type)) return PENDING;
  const status = PHASE_STATUS[phase];
  if (status) return status;
  return { key: "unknown", variant: "outline" };
}

/**
 * True while the service is mid-transition — the operator is building a release
 * or rolling one out, so its phase will change again shortly without anyone
 * touching the page. The detail header polls on this (w6/m46 t005): nothing in
 * the client re-reads `server(id)` when a deploy closes server-side, because
 * that fires no local mutation to hang a refetch off (w6/m45 t003 fixed only
 * the Cancel/Rollback BUTTON, which has one). An empty phase counts — a first
 * reconcile that has not landed is the start of the same transition.
 *
 * Deliberately keyed on the raw phase rather than on `deriveStatus`'s key, the
 * way the databases/keyvalue siblings are: `deriveStatus` folds a hibernated
 * non-HTTP service into "pending", and an auto-slept worker sitting at the
 * 3-second cadence forever is exactly what this must not do.
 */
export function isConvergingPhase(s: Pick<ServiceView, "phase">): boolean {
  return CONVERGING_PHASES.has(s.phase.toLowerCase());
}

// The App phases that are still moving, in the same lower-cased vocabulary
// PHASE_STATUS above is keyed on.
const CONVERGING_PHASES = new Set(["", "pending", "building", "deploying"]);

/**
 * True when the App is auto-sleeping (free-tier, idle past its TTL) rather than
 * manually suspended — the state that gets the "wakes on the next request" hint.
 * Never true for a type that serves no HTTP, which cannot be woken by a request.
 */
export function isSleeping(s: ServiceView): boolean {
  return deriveStatus(s).key === "sleeping";
}

/** Stat-tile counts computed from the live list (total / running / suspended). */
export function computeStats(services: ServiceView[]): ServiceStats {
  return {
    total: services.length,
    running: services.filter((s) => deriveStatus(s).key === "running").length,
    suspended: services.filter((s) => s.suspended).length,
  };
}

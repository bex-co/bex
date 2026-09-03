import { autoSleepEligible } from "@/features/services/lib/idle-timeout";
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
  // deriveStatus admits Hibernated here only for an eligible free web service;
  // it handles explicit suspension and ineligible service types first.
  hibernated: { key: "sleeping", variant: "secondary" },
  // Phase Canceled means the user stopped the release that was rolling before
  // any release had ever succeeded — so there is nothing running, but nothing
  // failed either. Non-destructive on purpose (w6/m52): the red "Failed" badge
  // it used to share told the user their own Cancel had broken something.
  canceled: { key: "canceled", variant: "secondary" },
  // Phase Deleting means the delete was accepted and the finalizer is tearing
  // the service down. By-id reads 404 the moment deletion is accepted (w3/m81),
  // so the dashboard normally redirects on the not-found rather than reaching
  // this — it keeps a mid-teardown service reading honestly (muted "Deleting",
  // not the generic red-herring "Unknown") for any transient window it is seen.
  deleting: { key: "deleting", variant: "secondary" },
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
  // Only a free public web service can be an activator-backed sleeper. For all
  // other services Hibernated is a manual-resume transition or stale state, so
  // promising "wakes on the next request" would be false.
  if (phase === "hibernated" && !autoSleepEligible(s.type, s.plan))
    return PENDING;
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
 * way the databases/keyvalue siblings are: `deriveStatus` folds an ineligible
 * Hibernated service into "pending", but that display fallback is not proof the
 * operator is actively converging and must not create an endless poll loop.
 */
export function isConvergingPhase(s: Pick<ServiceView, "phase">): boolean {
  return CONVERGING_PHASES.has(s.phase.toLowerCase());
}

// The App phases that are still moving, in the same lower-cased vocabulary
// PHASE_STATUS above is keyed on. "deleting" is included so a service observed
// mid-teardown keeps polling until the by-id read resolves to not-found (w3/m81)
// — which fires the detail route's redirect — instead of freezing on a stale
// cached row the way the m81 fixture did for 2+ hours.
const CONVERGING_PHASES = new Set([
  "",
  "pending",
  "building",
  "deploying",
  "deleting",
]);

/**
 * True when an eligible free public web App is auto-sleeping rather than
 * manually suspended — the state that gets the "wakes on the next request" hint.
 */
export function isSleeping(s: ServiceView): boolean {
  return deriveStatus(s).key === "sleeping";
}

/**
 * True while the service is being torn down (w3/m81). Keyed on the raw phase,
 * NOT `deriveStatus`'s key — `deriveStatus` folds suspension over phase, so a
 * suspended-then-deleting service would resolve to "suspended" and skip the
 * dead-URL suppression the header needs. Centralizes the phase-casing here (the
 * `isConvergingPhase` rule) so a caller never re-spells the "deleting" literal.
 */
export function isDeleting(s: Pick<ServiceView, "phase">): boolean {
  return s.phase.toLowerCase() === "deleting";
}

/** Stat-tile counts computed from the live list (total / running / suspended). */
export function computeStats(services: ServiceView[]): ServiceStats {
  return {
    total: services.length,
    running: services.filter((s) => deriveStatus(s).key === "running").length,
    suspended: services.filter((s) => s.suspended).length,
  };
}

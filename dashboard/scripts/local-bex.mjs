// local-bex.mjs — a tiny local stand-in for bex-api + Kratos, for developing the
// dashboard offline (no cluster, no Ory, no prod CORS/cookies).
//
// It speaks just enough of bex-api's wire protocol for the app to run:
//   • POST /graphql             — the reads the dashboard fires (services, server,
//                                 logs (with type/startTime/endTime windowing,
//                                 w9/m1/t002 parity), a single deploy by id
//                                 (w9/m1/t001 parity), managed-Postgres databases,
//                                 managed Key Value stores, and safe empties for
//                                 the rest)
//   • GET  /v1/logs/subscribe   — the SSE live-log tail (docs/ADR010-observability.md)
//   • GET  /sessions/whoami     — Kratos session check, so the auth guard passes
//   • GET  /sessions            — Kratos listMySessions (Settings → Active sessions)
//   • GET/DELETE /admin/oauth2/auth/sessions/consent — Hydra admin consent
//     sessions (Settings → Connected agents; dev:local sets HYDRA_ADMIN_URL here)
// CORS is wide-open (echoes the Origin, allows credentials) and there is NO auth —
// it is a DEV TOOL, never a real backend. Full-fidelity local bex needs the mock
// cluster + Ory stack (scripts/mock-cluster.sh); this is the fast frontend loop.
//
// Usage:
//   node scripts/local-bex.mjs            # listens on :8099
//   PORT=9000 node scripts/local-bex.mjs
// then run the dashboard pointed at it:
//   VITE_API_URL=http://localhost:8099/graphql \
//   VITE_KRATOS_PUBLIC_URL=http://localhost:8099 \
//   VITE_KRATOS_SSR_URL=http://localhost:8099 yarn dev

import { createServer } from "node:http";

const PORT = Number(process.env.PORT) || 8099;

// Two workspaces (w6/m1 "Workspace" shape), so the dashboard's switcher (w6/m3)
// has something to switch between and per-workspace scoping (ownerId filters,
// w6/m2/t004) is visibly exercised: the second workspace seeds NO
// services/databases, so switching to it should show empty lists, not a
// borrowed copy of the first workspace's resources.
const WORKSPACE_DEFAULT = "tea-localdefault00000001";
const WORKSPACE_SECOND = "tea-localsecond000000002";
const WORKSPACES = [
  {
    __typename: "Workspace",
    id: WORKSPACE_DEFAULT,
    name: "acme-hq",
    plan: "hobby",
    role: "admin",
    createdAt: "2026-06-01T09:00:00Z",
  },
  {
    __typename: "Workspace",
    id: WORKSPACE_SECOND,
    name: "acme-staging",
    plan: "pro",
    role: "admin",
    createdAt: "2026-06-15T09:00:00Z",
  },
];

// Render allows five free Hobby workspaces per user (w6/RESEARCH-workspaces.md
// finding 3) — mirrored here so the create flow's inline limit error
// (w6/m3 DoD) is reachable offline without a real control-plane store.
const HOBBY_WORKSPACE_CAP = 5;

// Workspace members (w6/m10: enriched userId + email). Three rows exercise
// every identity tier member-row.tsx falls through (email || userId || subject):
// a fully-enriched admin, a fully-enriched developer, and a viewer whose Kratos
// email lookup missed (userId still resolved, email empty — the "honest omit"
// degradation w6/m10's backend contract guarantees, not an error). ownerId +
// byOwner (below) scope these to a workspace, same as SERVICES/DATABASES; the
// second workspace seeds none, mirroring how it seeds no services/databases.
const WORKSPACE_MEMBERS = [
  {
    __typename: "WorkspaceMember",
    subject: "11111111-1111-4111-8111-111111111111",
    userId: "own-localmember0001",
    email: "owner@acme-hq.example",
    role: "ADMIN",
    createdAt: "2026-06-01T09:00:00Z",
    ownerId: WORKSPACE_DEFAULT,
  },
  {
    __typename: "WorkspaceMember",
    subject: "22222222-2222-4222-8222-222222222222",
    userId: "own-localmember0002",
    email: "dev@acme-hq.example",
    role: "DEVELOPER",
    createdAt: "2026-06-05T09:00:00Z",
    ownerId: WORKSPACE_DEFAULT,
  },
  {
    __typename: "WorkspaceMember",
    subject: "33333333-3333-4333-8333-333333333333",
    userId: "own-localmember0003",
    email: "",
    role: "VIEWER",
    createdAt: "2026-06-10T09:00:00Z",
    ownerId: WORKSPACE_DEFAULT,
  },
];

// Hydra admin consent sessions (w4/m18's "Connected agents" card): one OAuth2
// client the dev user has "authorized", so the Settings page's list + revoke
// flow works offline. Shape mirrors what @ory/client-fetch's
// listOAuth2ConsentSessions returns (consent_request.client + grant_scope).
const CONSENT_SESSIONS = [
  {
    grant_scope: ["openid", "offline_access", "bex:read", "bex:write"],
    handled_at: "2026-07-08T15:20:00Z",
    consent_request: {
      client: {
        client_id: "local-claude-code",
        client_name: "Claude Code",
        client_uri: "https://claude.com/claude-code",
      },
    },
  },
];

// One sample App, Render-shaped (matches the dashboard's Service selection set).
// ownerId scopes it to the default workspace — a stub-only field, harmless if
// it leaks into a response the dashboard didn't select it in.
const SERVICE = {
  __typename: "Service",
  id: "eden-cms-v2",
  name: "eden-cms-v2",
  displayName: null,
  type: "web_service",
  suspended: null,
  dashboardUrl: "http://localhost:5173/services/eden-cms-v2",
  url: "https://eden-cms-v2.onbex.co",
  createdAt: "2026-06-01T09:00:00Z",
  phase: "Running",
  replicas: 2,
  revision: "a1b2c3d",
  plan: "starter",
  schedule: null,
  command: null,
  runs: [],
  healthCheckPath: "/healthz",
  preDeployCommand: null,
  idleTTLSeconds: 0,
  repo: "https://github.com/acme-corp/eden-cms-v2",
  branch: "main",
  rootDir: "",
  autoDeploy: true,
  publishPath: null,
  routes: [],
  headers: [],
  ownerId: WORKSPACE_DEFAULT,
};

// Two more service types (w1/m15): a background_worker (no HTTP port/URL) and a
// cron_job (schedule + run history). Seeds so the type badge, the worker's
// no-URL detail, and the cron's schedule + recent-runs section all render.
const WORKER = {
  __typename: "Service",
  id: "email-worker",
  name: "email-worker",
  displayName: null,
  type: "background_worker",
  suspended: null,
  dashboardUrl: "http://localhost:5173/services/email-worker",
  url: null,
  createdAt: "2026-06-05T11:30:00Z",
  phase: "Running",
  replicas: 1,
  revision: "9f8e7d6",
  plan: "starter",
  schedule: null,
  command: null,
  runs: [],
  healthCheckPath: null,
  preDeployCommand: null,
  idleTTLSeconds: 0,
  repo: null,
  branch: null,
  rootDir: null,
  autoDeploy: null,
  publishPath: null,
  routes: [],
  headers: [],
  ownerId: WORKSPACE_DEFAULT,
};

const CRON = {
  __typename: "Service",
  id: "nightly-report",
  name: "nightly-report",
  displayName: null,
  type: "cron_job",
  suspended: null,
  dashboardUrl: "http://localhost:5173/services/nightly-report",
  url: null,
  createdAt: "2026-06-10T08:00:00Z",
  phase: "Running",
  replicas: null,
  revision: "3c2b1a0",
  plan: "free",
  schedule: "*/15 * * * *",
  command: "npm run send-nightly-report",
  runs: [
    {
      __typename: "CronRun",
      name: "nightly-report-run-8f21",
      startedAt: "2026-07-09T10:00:00Z",
      finishedAt: "2026-07-09T10:00:07Z",
      status: "Succeeded",
    },
    {
      __typename: "CronRun",
      name: "nightly-report-run-6d40",
      startedAt: "2026-07-09T09:45:00Z",
      finishedAt: "2026-07-09T09:45:31Z",
      status: "Failed",
    },
    {
      __typename: "CronRun",
      name: "nightly-report-run-4b19",
      startedAt: "2026-07-09T09:30:00Z",
      finishedAt: null,
      status: "Running",
    },
  ],
  healthCheckPath: null,
  preDeployCommand: null,
  idleTTLSeconds: 0,
  repo: null,
  branch: null,
  rootDir: null,
  autoDeploy: null,
  publishPath: null,
  routes: [],
  headers: [],
  ownerId: WORKSPACE_DEFAULT,
};

const SERVICES = [SERVICE, WORKER, CRON];

// Secret Deploy Hook URLs (w2/m33). These are synthetic dev-only values, kept
// per service so the Settings control can reveal/copy/rotate offline.
const DEPLOY_HOOKS = new Map(
  SERVICES.map((service) => [
    service.id,
    `http://localhost:${PORT}/v1/deploy-hooks/dhk-local-${service.id}-1`,
  ]),
);
let deployHookGeneration = 1;

// Per-service deploy-event history (w5/m7). Seeded with a few realistic entries
// for the web service; other services start empty. TriggerDeploy/Cancel/Rollback
// prepend to the appropriate list so the Events tab reflects mutations. Shape
// mirrors dashboard/src/features/services/api/events.graphql exactly: a flat
// ServiceEvent[] (each carrying its OWN cursor, no {cursor,events} envelope),
// and ServiceEventDetails is {deployStatus, actor, triggeredByUser, trigger:
// {firstBuild,envUpdated,manual,deployedByRender,clearCache,rollback}} — a
// different shape from the Deploy mutations' plain-string trigger/no envelope.
const EVENTS_BY_SERVICE = {
  "eden-cms-v2": [
    {
      __typename: "ServiceEvent",
      id: "dep-live-001",
      type: "deploy",
      timestamp: "2026-07-11T14:30:00Z",
      cursor: "dep-live-001",
      details: {
        __typename: "ServiceEventDetails",
        deployStatus: "live",
        preDeployStatus: "succeeded",
        actor: "dev@localhost",
        triggeredByUser: "dev@localhost",
        trigger: {
          __typename: "DeployTrigger",
          firstBuild: false,
          envUpdated: false,
          manual: true,
          deployedByRender: false,
          clearCache: false,
          rollback: false,
        },
      },
    },
    {
      __typename: "ServiceEvent",
      id: "dep-failed-000",
      type: "deploy",
      timestamp: "2026-07-11T13:00:00Z",
      cursor: "dep-failed-000",
      details: {
        __typename: "ServiceEventDetails",
        deployStatus: "update_failed",
        preDeployStatus: "failed",
        actor: "github",
        triggeredByUser: null,
        trigger: {
          __typename: "DeployTrigger",
          firstBuild: false,
          envUpdated: false,
          manual: false,
          deployedByRender: true,
          clearCache: false,
          rollback: false,
        },
      },
    },
  ],
};

// Per-service Deploy objects (w9/m1/t005), the `deploy(serviceId, deployId)`
// read's backing store — a separate shape from EVENTS_BY_SERVICE's
// ServiceEventDetails (deploys.graphql.deployGQLType's flat fields: id,
// status, trigger, image, rollbackOf, createdAt, startedAt, finishedAt,
// preDeployStatus). Seeded 1:1 with EVENTS_BY_SERVICE's two eden-cms-v2 rows
// (both terminal, hand-authored once, never mutated) so their Events-tab
// links resolve to a real deploy page. A deploy the mutation cases CREATE
// (TriggerDeploy/RollbackService) is different: its ServiceEvent is built by
// deployServiceEvent(), which reads status/preDeployStatus live off this same
// Deploy object via a getter — so CancelDeploy and the TriggerDeploy
// setTimeout only ever write the Deploy row, never a second copy.
const DEPLOYS_BY_SERVICE = {
  "eden-cms-v2": [
    {
      __typename: "Deploy",
      id: "dep-live-001",
      status: "live",
      trigger: "api",
      image: "registry.example.com/eden-cms-v2:a1b2c3d",
      rollbackOf: "",
      commitId: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678",
      commitMessage: "feat(cms): lazy-load the asset picker\n\nCuts editor TTI by ~400ms.",
      createdAt: "2026-07-11T14:30:00Z",
      startedAt: "2026-07-11T14:30:05Z",
      finishedAt: "2026-07-11T14:31:40Z",
      preDeployStatus: "succeeded",
    },
    {
      __typename: "Deploy",
      id: "dep-failed-000",
      status: "update_failed",
      trigger: "api",
      image: "registry.example.com/eden-cms-v2:9f8e7d6",
      rollbackOf: "",
      commitId: "9f8e7d6c5b4a30291827364554637281909aebcd",
      commitMessage: "chore: bump image base",
      createdAt: "2026-07-11T13:00:00Z",
      startedAt: "2026-07-11T13:00:04Z",
      finishedAt: "2026-07-11T13:01:12Z",
      preDeployStatus: "failed",
    },
  ],
};

// Look up one service by id, or null — the stub must NEVER fabricate a service for
// an unknown id (the 2026-07-09 phantom-service bug: any id echoed eden-cms-v2).
function serviceById(id) {
  return SERVICES.find((s) => s.id === id) ?? null;
}

// The deploy list for a service (empty array for a service with none / unknown id).
function deploysFor(serviceId) {
  return DEPLOYS_BY_SERVICE[serviceId] ?? [];
}

// Look up one deploy scoped to a service, or null — mirrors serviceById's
// never-borrow discipline: a deployId belonging to a DIFFERENT service (or an
// unknown deployId) must not resolve, exactly like the real Get verb
// (deploys.Service.Get, w9/m1/t001's not-found contract).
function deployById(serviceId, deployId) {
  return deploysFor(serviceId).find((d) => d.id === deployId) ?? null;
}

// Resolve the deploy a windowed build/predeploy Logs query is asking about.
// The dashboard always sends the deploy's OWN createdAt as `startTime` (t003's
// useDeploy → useDeployLogs wiring), so an exact match is enough for a stub —
// no need to model interval overlap.
function deployForWindow(serviceId, startTime) {
  return deploysFor(serviceId).find((d) => d.createdAt === startTime) ?? null;
}

// Synthetic build + pre-deploy log lines for one seeded/triggered deploy
// (w9/m1/t005) — a few scripted "==> …" build steps, and a pre-deploy
// succeeded/failed line when the deploy has a preDeployStatus. Timestamps
// land a few hundred ms apart starting at the deploy's createdAt, well inside
// its own window.
function deploySyntheticLogs(deploy) {
  const base = Date.parse(deploy.createdAt);
  const entry = (offsetMs, message, type) => ({
    __typename: "LogEntry",
    timestamp: new Date(base + offsetMs).toISOString(),
    message,
    type,
    instance: null,
    level: null,
    method: null,
    statusCode: null,
  });
  const build = [
    entry(0, "==> Cloning from GitHub…", "build"),
    entry(300, "==> Using Node.js version 20.11.0", "build"),
    entry(700, "==> Running build command 'yarn build'…", "build"),
    entry(1600, "==> Build successful", "build"),
    entry(1900, "==> Uploading build…", "build"),
  ];
  const predeploy = deploy.preDeployStatus
    ? [
        entry(2200, "Running pre-deploy command…", "predeploy"),
        entry(
          2600,
          deploy.preDeployStatus === "failed"
            ? "pre-deploy command exited 1"
            : "pre-deploy command exited 0",
          "predeploy",
        ),
      ]
    : [];
  return { build, predeploy };
}

// The terminal statuses a deploy row settles into (store.DeployLive/
// DeployUpdateFailed/DeployCanceled) — a deploy created in one of these is
// already finished (RollbackService's immediate "live"); anything else means
// finishedAt stays null until a mutation (the TriggerDeploy setTimeout,
// CancelDeploy) closes it.
const TERMINAL_DEPLOY_STATUSES = new Set(["live", "update_failed", "canceled"]);

// makeDeploy builds a Deploy row (w9/m1/t005) — the shape TriggerDeploy and
// RollbackService both need, differing only in status/trigger/rollbackOf/
// image/preDeployStatus. Push the RETURNED object straight into
// DEPLOYS_BY_SERVICE and reference it (never copy its fields) from the
// matching ServiceEvent via deployServiceEvent below, so there is exactly one
// place a status transition needs to be written.
function makeDeploy({ id, status, trigger, rollbackOf = "", image, preDeployStatus = "", commitId = "", commitMessage = "" }) {
  const createdAt = new Date().toISOString();
  return {
    __typename: "Deploy",
    id,
    status,
    createdAt,
    startedAt: createdAt,
    finishedAt: TERMINAL_DEPLOY_STATUSES.has(status) ? createdAt : null,
    trigger,
    rollbackOf,
    image,
    preDeployStatus,
    commitId,
    commitMessage,
  };
}

// deployServiceEvent projects a Deploy row onto the Events-tab shape
// (EVENTS_BY_SERVICE's ServiceEvent, a different shape from Deploy: boolean
// trigger flags instead of a plain string, no rollbackOf/image). `details`
// is a getter reading LIVE off `deploy` — mirroring how the real backend's
// events feed is a read-time view over the deploys table (internal/events),
// never a second written copy — so a later status transition only has to
// touch the Deploy row (see the TriggerDeploy/CancelDeploy cases) and every
// ServiceEvents read reflects it automatically.
function deployServiceEvent(deploy, trigger) {
  return {
    __typename: "ServiceEvent",
    id: deploy.id,
    type: "deploy",
    timestamp: deploy.createdAt,
    cursor: deploy.id,
    get details() {
      return {
        __typename: "ServiceEventDetails",
        deployStatus: deploy.status,
        preDeployStatus: deploy.preDeployStatus,
        actor: "dev@localhost",
        triggeredByUser: "dev@localhost",
        trigger,
      };
    },
  };
}

// Per-service synthetic instance ids (replicas), derived from the service id so
// each service's logs are visibly its own (no cross-service bleed).
function instancesFor(resource) {
  return [`${resource}-7d9f8-abcde`, `${resource}-7d9f8-fghij`];
}

// The platform host a custom domain CNAMEs to: <service>.onbex.co, mirroring the
// backend's `<app>.<BEX_BASE_DOMAIN>` target (docs/ADR005-custom-domain.md).
function platformHostFor(serviceId) {
  return `${serviceId}.onbex.co`;
}

// apex (2 labels, e.g. example.com) vs subdomain — the backend's domainType heuristic.
function domainTypeFor(name) {
  return name.split(".").length <= 2 ? "apex" : "subdomain";
}

// The DNS record the tenant must create (backend DNSRecordView, w5/m10): a CNAME
// from the subdomain label prefix, or an ALIAS at @ for an apex, pointing at the
// platform host. Mirrors lego/backend/internal/apps/domains.go dnsRecordFor.
function dnsRecordFor(name, platformHost) {
  if (domainTypeFor(name) === "apex") {
    return {
      __typename: "DNSRecord",
      type: "ALIAS",
      name: "@",
      value: platformHost,
    };
  }
  // Subdomain (>= 3 labels here — apex returned above): strip the trailing two
  // labels (the root zone) to get the record name (www.example.com -> "www").
  const recordName = name.split(".").slice(0, -2).join(".");
  return {
    __typename: "DNSRecord",
    type: "CNAME",
    name: recordName,
    value: platformHost,
  };
}

// Build a full CustomDomain object (Render shape + the w5/m10 dnsRecord) for a
// service. verificationStatus/serverStatus default to pending (just added).
function makeDomain(serviceId, name, over = {}) {
  return {
    __typename: "CustomDomain",
    id: name,
    name,
    domainType: domainTypeFor(name),
    verificationStatus: "pending",
    serverStatus: "pending",
    dnsRecord: dnsRecordFor(name, platformHostFor(serviceId)),
    ...over,
  };
}

// In-memory custom-domains store, keyed BY SERVICE (Render dashboard CustomDomain
// shape, w1/m11.5 + w5/m10 DNS instructions) so the Settings tab's
// list/add/delete/verify renders offline and each service owns its own domains.
// Only the web service (eden-cms-v2) is seeded — a worker/cron has no ingress, so
// its domain list is empty, making cross-service bleed visually obvious.
const CUSTOM_DOMAINS_BY_SERVICE = {
  "eden-cms-v2": [
    makeDomain("eden-cms-v2", "www.eden-cms.com", {
      verificationStatus: "verified",
      serverStatus: "active",
    }),
    makeDomain("eden-cms-v2", "eden-cms.com"),
  ],
};

// The domain list for a service (empty array for a service with none / unknown id).
function domainsFor(serviceId) {
  return CUSTOM_DOMAINS_BY_SERVICE[serviceId] ?? [];
}

// The managed-Postgres tier catalog (backend databaseInstanceTypes, w5/m8) — the
// create dialog's plan picker source. Kept in sync with lego/types/tiers.yaml's
// postgres family so the offline stub renders the same plans as the real API.
const DB_INSTANCE_TYPES = [
  {
    __typename: "DatabaseInstanceType",
    id: "free",
    name: "Free",
    cpu: "100m",
    memory: "256Mi",
    storageGB: 1,
  },
  {
    __typename: "DatabaseInstanceType",
    id: "basic-256mb",
    name: "Basic 256MB",
    cpu: "100m",
    memory: "256Mi",
    storageGB: 1,
  },
  {
    __typename: "DatabaseInstanceType",
    id: "basic-1gb",
    name: "Basic 1GB",
    cpu: "500m",
    memory: "1Gi",
    storageGB: 5,
  },
];

// In-memory managed-Postgres store (Render dashboard `database` shape) so the
// Databases page's create/list/detail/delete + on-demand connection-info are
// interactive offline. Seeded with one available DB so the list isn't empty.
function makeDatabase(over = {}) {
  const name = over.name ?? "orders-db";
  const dbn = name.toLowerCase().replaceAll("-", "_");
  return {
    __typename: "Database",
    id: name,
    name,
    plan: "basic-1gb",
    version: "16",
    status: "available",
    databaseName: dbn,
    databaseUser: `${dbn}_user`,
    diskSizeGB: 5,
    highAvailabilityEnabled: false,
    readReplicas: [],
    suspended: "not_suspended",
    createdAt: "2026-06-20T10:00:00Z",
    externalHost: "orders-db.db.bex.co",
    public: true,
    poolerEnabled: false,
    backupsEnabled: false,
    ipAllowList: [],
    ownerId: WORKSPACE_DEFAULT,
    ...over,
  };
}
const DATABASES = [makeDatabase()];

// The managed Key Value tier catalog (backend keyValueInstanceTypes, w5/m12) —
// the create form's plan picker source. Kept in sync with lego/types/tiers.yaml's
// Valkey family so the offline stub renders the same plans as the real API.
const KV_INSTANCE_TYPES = [
  {
    __typename: "KeyValueInstanceType",
    id: "free",
    name: "Free",
    cpu: "100m",
    memory: "128Mi",
    storageGB: 1,
  },
  {
    __typename: "KeyValueInstanceType",
    id: "starter",
    name: "Starter",
    cpu: "100m",
    memory: "256Mi",
    storageGB: 1,
  },
  {
    __typename: "KeyValueInstanceType",
    id: "standard",
    name: "Standard",
    cpu: "500m",
    memory: "1Gi",
    storageGB: 5,
  },
];

// In-memory managed Key Value store (Render dashboard `keyValue` shape) so the
// Key Value page's create/list/detail/delete/suspend/resume + on-demand
// connection-info are interactive offline. Seeded with one available store
// (so the list/detail/connection-info paths render) and one still converging
// (so the "creating" chip + gated poll are exercised, per t002's fixture ask).
function makeKeyValue(over = {}) {
  const name = over.name ?? "sessions-cache";
  return {
    __typename: "KeyValue",
    id: name,
    name,
    plan: "starter",
    version: "8",
    status: "available",
    suspended: "not_suspended",
    createdAt: "2026-06-22T14:00:00Z",
    externalHost: "sessions-cache.kv.bex.co",
    public: true,
    ...over,
  };
}
const KEY_VALUES = [
  makeKeyValue(),
  makeKeyValue({
    name: "rate-limiter",
    plan: "free",
    version: "",
    status: "creating",
    externalHost: "",
    public: false,
    createdAt: "2026-07-09T21:00:00Z",
  }),
];

// Projects (w1/m31, bex extension — an interactive in-memory store, mirroring
// Databases/KeyValues above). One seeded project spanning all three resource
// kinds (a service + a database) so the unified dashboard Projects page's
// merged grouping is visibly exercised offline; "rate-limiter" and the
// worker/cron services are left ungrouped for the "No Project" section.
const PROJECTS = [
  {
    __typename: "Project",
    id: "prj-local0001",
    name: "storefront",
    ownerId: WORKSPACE_DEFAULT,
    createdAt: "2026-06-25T09:00:00Z",
    serviceIds: ["eden-cms-v2"],
    databaseIds: ["orders-db"],
    keyValueIds: ["sessions-cache"],
  },
];

// Environments (w1/m32, bex extension — docs/ADR032-environments.md): named
// subsets of a project's services. Seeded empty so the project page shows the
// "no environments yet" state first; Create/Rename/Delete/SetServices mutate
// ENVIRONMENTS in place. SetEnvironmentServices auto-joins the assigned
// services to the parent project and (single-column membership) evicts them
// from any other environment — the same side effects the real store applies.
const ENVIRONMENTS = [];

// Workspace audit trail (w4/m10 surface, w6/m14 workspace scoping) — the shape
// of internal/audit/graphql.go's AuditLog type. Every event is tagged with the
// workspace that owns it, and the two workspaces' events are deliberately
// UNMISTAKABLE from each other ("alpha-*" resources in acme-hq, "bravo-*" in
// acme-staging): the whole point of the Settings → Security & Compliance audit
// table is that it follows the switcher, so a stub that seeded look-alike rows
// would make the one bug it must catch (the table pinned to workspaces[0])
// invisible. Newest-first, like the real keyset-paged store.
function auditEvent(ownerId, n, over = {}) {
  return {
    __typename: "AuditLog",
    id: `aud-${ownerId.slice(4, 12)}-${String(n).padStart(4, "0")}`,
    timestamp: new Date(
      Date.UTC(2026, 6, 10, 12, 0, 0) - n * 3600_000,
    ).toISOString(),
    actor: "dev@localhost",
    actorMethod: "session",
    action: "update",
    status: "success",
    resource: "",
    ownerId,
    ...over,
  };
}

const AUDIT_EVENTS = [
  // acme-hq (WORKSPACE_DEFAULT) — every resource says "alpha".
  auditEvent(WORKSPACE_DEFAULT, 1, {
    action: "deploy",
    resource: "alpha-web/srv-alpha0001",
  }),
  auditEvent(WORKSPACE_DEFAULT, 2, {
    action: "update_env",
    resource: "alpha-web/srv-alpha0001",
  }),
  auditEvent(WORKSPACE_DEFAULT, 3, {
    action: "delete",
    resource: "alpha-cache/kv-alpha0002",
    status: "denied",
    actor: "viewer@alpha.example",
  }),
  auditEvent(WORKSPACE_DEFAULT, 4, {
    action: "create",
    resource: "alpha-db/dbs-alpha0003",
    actorMethod: "api_key",
    actor: "key-alpha-ci",
  }),
  auditEvent(WORKSPACE_DEFAULT, 5, {
    action: "suspend",
    resource: "alpha-worker/srv-alpha0004",
  }),
  // acme-staging (WORKSPACE_SECOND) — every resource says "bravo".
  auditEvent(WORKSPACE_SECOND, 1, {
    action: "restart",
    resource: "bravo-api/srv-bravo0001",
  }),
  auditEvent(WORKSPACE_SECOND, 2, {
    action: "rotate_key",
    resource: "bravo-api/srv-bravo0001",
    actorMethod: "api_key",
    actor: "key-bravo-bot",
  }),
  auditEvent(WORKSPACE_SECOND, 3, {
    action: "scale",
    resource: "bravo-batch/srv-bravo0002",
    status: "denied",
    actor: "intern@bravo.example",
  }),
];

// Newest-first, workspace-scoped, keyset-paged past `cursor` (the last returned
// event's id), honoring `limit` — the contract internal/store/audit.go exposes.
function auditLogsFor({ ownerId, cursor, limit }) {
  let rows = byOwner(AUDIT_EVENTS, ownerId).sort((a, b) =>
    b.timestamp.localeCompare(a.timestamp),
  );
  if (cursor) {
    const i = rows.findIndex((e) => e.id === cursor);
    rows = i >= 0 ? rows.slice(i + 1) : rows;
  }
  return rows.slice(0, limit ?? 20);
}

// A pool of realistic-looking app log messages the generator draws from.
const MESSAGES = [
  'level=info msg="GET /api/health 200" duration=1.2ms',
  'level=info msg="GET /api/ledger/accounts 200" duration=18.4ms rows=42',
  'level=info msg="POST /api/entries 201" duration=27.9ms',
  'level=warn msg="slow query" table=transactions duration=812ms',
  'level=info msg="cache hit" key=reports:overview:2026-07',
  'level=info msg="GET /api/reports/overview 200" duration=64.1ms',
  'level=error msg="upstream timeout" service=fx-rates attempt=2',
  'level=info msg="reconnected" service=fx-rates',
  'level=info msg="GET /assets/app.css 304" duration=0.4ms',
  'level=info msg="scheduler tick" jobs=3 pending=0',
];

function line(iso, i, resource) {
  const instances = instancesFor(resource);
  const message = MESSAGES[i % MESSAGES.length];
  const levelMatch = /level=(\w+)/.exec(message);
  return {
    __typename: "LogEntry",
    timestamp: iso,
    message,
    type: "app",
    instance: instances[i % instances.length],
    level: levelMatch ? levelMatch[1] : "info",
    method: null,
    statusCode: null,
  };
}

// A backfill of historical lines, oldest-first, ending ~now — scoped to the
// requested resource (service) so each service's history is its own.
function history(count, resource) {
  const now = Date.now();
  const out = [];
  for (let i = count - 1; i >= 0; i--) {
    out.push(
      line(new Date(now - i * 3000).toISOString(), count - 1 - i, resource),
    );
  }
  return out;
}

// Render-shaped SSE frame (internal/logs/render.go): id + message + timestamp +
// a [{name,value}] labels array. Labelled with the requested resource. `level`
// is parsed from the synthetic message's own `level=X` logfmt field (the real
// backend parses a structured JSON severity field instead — this is just
// enough for the dev stub's live-tail merge, dashboard/src/features/logs/lib/map.ts,
// to have a level label to read, matching what the history/GraphQL path sends).
function renderLog(entry, seq, resource) {
  const levelMatch = /level=(\w+)/.exec(entry.message);
  return {
    id: `${entry.instance}-${entry.timestamp}-${seq}`,
    message: entry.message,
    timestamp: entry.timestamp,
    labels: [
      { name: "type", value: "app" },
      { name: "resource", value: resource },
      { name: "instance", value: entry.instance },
      { name: "level", value: levelMatch ? levelMatch[1] : "info" },
    ],
  };
}

function cors(req, res) {
  res.setHeader("Access-Control-Allow-Origin", req.headers.origin || "*");
  res.setHeader("Vary", "Origin");
  res.setHeader("Access-Control-Allow-Credentials", "true");
  res.setHeader(
    "Access-Control-Allow-Headers",
    "Authorization, Content-Type, X-Session-Token",
  );
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
}

function json(res, code, body) {
  res.writeHead(code, { "Content-Type": "application/json" });
  res.end(JSON.stringify(body));
}

// ownerId scopes a list to one workspace (w6/m2/t004); the dashboard's
// switcher (w6/m3) always passes the selected workspace's id once it
// resolves, so an unseeded workspace correctly shows an empty list.
function byOwner(list, ownerId) {
  return list.filter((x) => x.ownerId === (ownerId || WORKSPACE_DEFAULT));
}

// GitHub App connection stub (w5/m15 create wizard): a pre-connected account
// so the repo picker renders populated. Set connected: false to test the
// connect-prompt empty state (see the GitConnection query, internal/github/graphql.go).
const GIT_CONNECTION = {
  __typename: "GitConnection",
  connected: true,
  accountLogin: "acme-corp",
  installationId: 87654321,
  createdAt: "2026-06-01T09:00:00Z",
  installUrl: "https://github.com/apps/bex-local/installations/new",
};

// Sample repos visible to the connected GitHub App installation (w5/m15):
// a mix of public and private repos from the same org, with different default
// branches so the branch auto-fill and UI label are exercised end-to-end.
const REPOS = [
  {
    __typename: "Repo",
    id: 1001,
    fullName: "acme-corp/web-frontend",
    private: false,
    defaultBranch: "main",
    htmlUrl: "https://github.com/acme-corp/web-frontend",
    cloneUrl: "https://github.com/acme-corp/web-frontend.git",
  },
  {
    __typename: "Repo",
    id: 1002,
    fullName: "acme-corp/api-service",
    private: true,
    defaultBranch: "main",
    htmlUrl: "https://github.com/acme-corp/api-service",
    cloneUrl: "https://github.com/acme-corp/api-service.git",
  },
  {
    __typename: "Repo",
    id: 1003,
    fullName: "acme-corp/data-pipeline",
    private: true,
    defaultBranch: "develop",
    htmlUrl: "https://github.com/acme-corp/data-pipeline",
    cloneUrl: "https://github.com/acme-corp/data-pipeline.git",
  },
  {
    __typename: "Repo",
    id: 1004,
    fullName: "acme-corp/marketing-site",
    private: false,
    defaultBranch: "main",
    htmlUrl: "https://github.com/acme-corp/marketing-site",
    cloneUrl: "https://github.com/acme-corp/marketing-site.git",
  },
  {
    __typename: "Repo",
    id: 1005,
    fullName: "acme-corp/internal-tools",
    private: true,
    defaultBranch: "master",
    htmlUrl: "https://github.com/acme-corp/internal-tools",
    cloneUrl: "https://github.com/acme-corp/internal-tools.git",
  },
];

// Resolve one GraphQL operation to canned data. Only the reads the dashboard
// actually fires need real shapes; everything else returns a safe empty.
function resolveGraphQL({ operationName, variables = {} }) {
  switch (operationName) {
    case "Services":
      return { services: byOwner(SERVICES, variables.ownerId) };
    case "GitConnection":
      return { gitConnection: GIT_CONNECTION };
    case "Repos":
      // Only return repos when connected; an unconnected installation has no repos.
      return { repos: GIT_CONNECTION.connected ? REPOS : [] };
    case "ConnectGit":
      // Simulate the GitHub App install flow completing: mark as connected and
      // return the install URL (in practice the user visits it and comes back).
      GIT_CONNECTION.connected = true;
      return { connectGit: GIT_CONNECTION };
    case "CreateService": {
      const svc = {
        __typename: "Service",
        id: variables.name,
        name: variables.name,
        displayName: null,
        type: variables.image ? "web_service" : "web_service",
        suspended: null,
        dashboardUrl: `http://localhost:5173/services/${variables.name}`,
        url: `https://${variables.name}.onbex.co`,
        createdAt: new Date().toISOString(),
        phase: "Pending",
        replicas: 1,
        revision: null,
        plan: variables.plan ?? "free",
        schedule: null,
        command: null,
        runs: [],
        healthCheckPath: null,
        preDeployCommand: null,
        ownerId: WORKSPACE_DEFAULT,
        repo: variables.repo ?? null,
        branch: variables.branch ?? null,
        rootDir: variables.rootDir ?? null,
        image: variables.image ?? null,
      };
      SERVICES.push(svc);
      DEPLOY_HOOKS.set(
        svc.id,
        `http://localhost:${PORT}/v1/deploy-hooks/dhk-local-${svc.id}-1`,
      );
      // Simulate async rollout: transition Pending → Building → Running.
      setTimeout(() => {
        svc.phase = "Building";
      }, 2000);
      setTimeout(() => {
        svc.phase = "Running";
      }, 6000);
      return { createService: svc };
    }
    case "Server":
      // null for an unknown id — never borrow another service's object.
      return { server: serviceById(variables.id) };
    case "DeployHook": {
      const service = serviceById(variables.serviceId);
      return {
        deployHook: service
          ? { __typename: "DeployHook", url: DEPLOY_HOOKS.get(service.id) }
          : null,
      };
    }
    case "RegenerateDeployHook": {
      const service = serviceById(variables.serviceId);
      if (!service) throw new Error("not found");
      deployHookGeneration += 1;
      const url = `http://localhost:${PORT}/v1/deploy-hooks/dhk-local-${service.id}-${deployHookGeneration}`;
      DEPLOY_HOOKS.set(service.id, url);
      return {
        regenerateDeployHook: { __typename: "DeployHook", url },
      };
    }
    case "SetDisplayName": {
      const svc = serviceById(variables.id);
      if (!svc) throw new Error("not found");
      svc.displayName = String(variables.displayName ?? "").trim() || null;
      return { setDisplayName: svc };
    }
    case "DeleteService": {
      // Danger-zone delete (w5/m14), mirroring DeleteDatabase: drop the service
      // from the in-memory store so a subsequent Services list omits it.
      const i = SERVICES.findIndex((s) => s.id === variables.id);
      if (i >= 0) SERVICES.splice(i, 1);
      DEPLOY_HOOKS.delete(variables.id);
      return { deleteService: true };
    }
    case "ScaleService": {
      // Manual instance-count scaling (w5/m16); mirrors backend's 1–100 bounds.
      const n = variables.numInstances;
      if (n < 1 || n > 100) throw new Error("numInstances must be 1-100");
      const svc = serviceById(variables.id);
      if (svc) svc.replicas = n;
      return { scaleService: svc ?? null };
    }
    case "Logs": {
      // Honor the same filters bex-api honors: type + text substring +
      // startTime/endTime (w9/m1/t002 GraphQL parity — the deploy page's
      // windowed query).
      const type = variables.type;
      const resource = variables.resource ?? "";
      const limit = variables.limit ?? 100;
      const inWindow = (iso) => {
        const t = Date.parse(iso);
        if (variables.startTime && t < Date.parse(variables.startTime))
          return false;
        if (variables.endTime && t > Date.parse(variables.endTime))
          return false;
        return true;
      };
      // build/predeploy (w9/m1/t005): sourced from the seeded/triggered deploy
      // whose window the query names, never the synthetic app-log generator —
      // an unmatched window (unknown deploy) answers empty, never borrowed data.
      if (type === "build" || type === "predeploy") {
        const deploy = deployForWindow(resource, variables.startTime);
        const lines = deploy ? deploySyntheticLogs(deploy)[type] : [];
        return { logs: lines.filter((l) => inWindow(l.timestamp)).slice(-limit) };
      }
      if (type && type !== "app" && type !== "application") return { logs: [] };
      let logs = history(60, resource).filter((l) => inWindow(l.timestamp));
      if (variables.text) {
        const q = String(variables.text).toLowerCase();
        logs = logs.filter((l) => l.message.toLowerCase().includes(q));
      }
      return { logs: logs.slice(-limit) };
    }
    // Single-deploy read (w9/m1/t001 GraphQL parity): the deploy detail page's
    // header data. Unknown deployId, or one belonging to a different service,
    // resolves null — never a borrowed deploy (deployById's contract above).
    case "Deploy":
      return { deploy: deployById(variables.serviceId, variables.deployId) };
    // Deploy-history list (w9/002, the dedicated Deploys tab): status[] +
    // keyset cursor + limit over the same per-service rows, newest-first —
    // mirrors internal/store's pageNewestFirst contract (unknown cursor =>
    // empty page, never the unfiltered list).
    case "Deploys": {
      let rows = deploysFor(variables.serviceId);
      const statuses = (variables.status ?? []).filter(Boolean);
      if (statuses.length > 0) rows = rows.filter((d) => statuses.includes(d.status));
      if (variables.cursor) {
        const at = rows.findIndex((d) => d.id === variables.cursor);
        rows = at === -1 ? [] : rows.slice(at + 1);
      }
      const limit = variables.limit ?? 0;
      if (limit > 0) rows = rows.slice(0, limit);
      return { deploys: rows };
    }
    // Logs-tab filter dropdowns (w5/008): observed label values for one App.
    // `level`/`instance` are the two labels the synthetic app-log generator
    // actually varies (see MESSAGES/instancesFor above); the request-log-only
    // labels (method/statusCode/host/type) have no bex-stub source, so they
    // answer empty rather than fabricate values the real store would 503 on.
    case "LogLabelValues": {
      const values =
        variables.label === "level"
          ? ["info", "warn", "error"]
          : variables.label === "instance"
            ? instancesFor(variables.resource ?? "")
            : variables.label === "type"
              ? ["app"]
              : [];
      return { logLabelValues: values };
    }
    // Tabs the Logs flow doesn't need — answer empty so nothing errors if visited.
    case "MetricsFilters":
      return {
        metricsFilters: { __typename: "MetricsFiltersResult", values: [] },
      };
    case "Metrics":
      return { metrics: [] };
    case "MonthToDateBandwidth":
      return { monthToDateBandwidth: null };
    // Scaling tab (w1/m20): no autoscaling configured for any stub service.
    case "AutoscalingConfig":
      return { autoscalingConfig: null };
    // Workspace-scoped month-to-date usage (w8/m2–m3 + m6): one entry per service
    // per kind. instance_seconds drives the Compute section; egress_bytes drives
    // Bandwidth; build_seconds drives Build Minutes. Values vary by period so the
    // trend view shows distinct bars across the last 3 months.
    case "Usage": {
      const period =
        variables.period ||
        (() => {
          const d = new Date();
          return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
        })();
      // Scale values by month offset so historical periods show lower totals.
      const [, mm] = period.split("-").map(Number);
      const currentMm = new Date().getMonth() + 1;
      const monthsBack = (currentMm - mm + 12) % 12;
      const scale = Math.max(0.3, 1 - monthsBack * 0.25);
      // Compute a synthetic estimatedCost matching the price sheet (pricing.yaml):
      // starter compute: $4.90/mo per 2,628,000 s; egress: $0.01/GB; build: $0.0035/min
      const starterSecs = Math.round((432000 + 216000) * scale); // eden-cms + email-worker
      const egressBytes = Math.round(524288000 * scale);
      const buildSecs = Math.round((1800 + 900 + 300) * scale);
      const computeCost = (starterSecs * 0.000001864554).toFixed(2);
      const bandwidthCost = (egressBytes * 0.000000000009313226).toFixed(2);
      const buildCost = (buildSecs * 0.000058333333).toFixed(2);
      const totalCost = (
        starterSecs * 0.000001864554 +
        egressBytes * 0.000000000009313226 +
        buildSecs * 0.000058333333
      ).toFixed(2);
      const meters = [
        parseFloat(computeCost) >= 0.005
          ? {
              __typename: "MeterEstimate",
              kind: "instance_seconds",
              tier: "starter",
              resourceKind: "service",
              costUsd: computeCost,
            }
          : null,
        parseFloat(bandwidthCost) >= 0.005
          ? {
              __typename: "MeterEstimate",
              kind: "egress_bytes",
              tier: "",
              resourceKind: "service",
              costUsd: bandwidthCost,
            }
          : null,
        parseFloat(buildCost) >= 0.005
          ? {
              __typename: "MeterEstimate",
              kind: "build_seconds",
              tier: "",
              resourceKind: "service",
              costUsd: buildCost,
            }
          : null,
      ].filter(Boolean);
      return {
        usage: {
          __typename: "UsageSummary",
          workspaceId: "local-workspace",
          period,
          services: [
            {
              __typename: "ServiceUsage",
              serviceId: "eden-cms-v2",
              resourceKind: "service",
              rows: [
                {
                  __typename: "UsageRow",
                  kind: "instance_seconds",
                  tier: "starter",
                  total: Math.round(432000 * scale),
                },
                {
                  __typename: "UsageRow",
                  kind: "egress_bytes",
                  tier: "",
                  total: Math.round(524288000 * scale),
                },
                {
                  __typename: "UsageRow",
                  kind: "build_seconds",
                  tier: "",
                  total: Math.round(1800 * scale),
                },
              ],
            },
            {
              __typename: "ServiceUsage",
              serviceId: "email-worker",
              resourceKind: "service",
              rows: [
                {
                  __typename: "UsageRow",
                  kind: "instance_seconds",
                  tier: "starter",
                  total: Math.round(216000 * scale),
                },
                {
                  __typename: "UsageRow",
                  kind: "build_seconds",
                  tier: "",
                  total: Math.round(900 * scale),
                },
              ],
            },
            {
              __typename: "ServiceUsage",
              serviceId: "nightly-report",
              resourceKind: "service",
              rows: [
                {
                  __typename: "UsageRow",
                  kind: "instance_seconds",
                  tier: "free",
                  total: Math.round(3600 * scale),
                },
                {
                  __typename: "UsageRow",
                  kind: "build_seconds",
                  tier: "",
                  total: Math.round(300 * scale),
                },
              ],
            },
          ],
          estimatedCost: {
            __typename: "EstimatedCost",
            totalUsd: totalCost,
            meters,
          },
        },
      };
    }
    case "InstanceTypes":
      return {
        instanceTypes: [
          {
            __typename: "InstanceType",
            id: "free",
            name: "Free",
            cpu: "0.1",
            memory: "512Mi",
          },
          {
            __typename: "InstanceType",
            id: "starter",
            name: "Starter",
            cpu: "0.5",
            memory: "1Gi",
          },
          {
            __typename: "InstanceType",
            id: "standard",
            name: "Standard",
            cpu: "1",
            memory: "2Gi",
          },
          {
            __typename: "InstanceType",
            id: "pro",
            name: "Pro",
            cpu: "2",
            memory: "4Gi",
          },
        ],
      };
    case "EnvVarKeys":
      return {
        service: { __typename: "Service", id: variables.id, envVarKeys: [] },
      };
    case "SecretFileNames":
      return {
        service: {
          __typename: "Service",
          id: variables.id,
          secretFileNames: [],
        },
      };
    case "EnvGroups":
      return { envGroups: [] };
    // Workspace lifecycle (w6/m1 verbs, w6/m3 dashboard UX) — an interactive
    // in-memory store so the switcher/create/rename/delete flow is exercisable
    // offline, including the Hobby-plan-cap inline error.
    case "Workspaces":
      return { workspaces: WORKSPACES };
    case "CreateWorkspace": {
      const plan = variables.plan || "hobby";
      if (plan === "hobby") {
        const hobbyCount = WORKSPACES.filter((w) => w.plan === "hobby").length;
        if (hobbyCount >= HOBBY_WORKSPACE_CAP) {
          throw new Error(
            `bad request: at most ${HOBBY_WORKSPACE_CAP} hobby workspaces per user`,
          );
        }
      }
      const created = {
        __typename: "Workspace",
        id: `tea-local${WORKSPACES.length}${Date.now().toString(36)}`,
        name: variables.name,
        plan,
        role: "admin",
        createdAt: new Date().toISOString(),
      };
      WORKSPACES.push(created);
      return { createWorkspace: created };
    }
    case "RenameWorkspace": {
      const w = WORKSPACES.find((ws) => ws.id === variables.id);
      if (!w) throw new Error("not found");
      w.name = variables.name;
      return { renameWorkspace: w };
    }
    case "DeleteWorkspace": {
      const w = WORKSPACES.find((ws) => ws.id === variables.id);
      if (!w) throw new Error("not found");
      // Render's live delete guard: "sudo delete workspace <name>", mirroring the
      // backend's DeleteConfirmation helper (backend/internal/workspaces/service.go,
      // w6/m5/t002, docs/render-artifacts/workspace-lifecycle.md).
      const want = `sudo delete workspace ${w.name}`;
      if (variables.confirmation !== want) {
        throw new Error(`bad request: confirmation must be "${want}"`);
      }
      WORKSPACES.splice(WORKSPACES.indexOf(w), 1);
      return { deleteWorkspace: w.id };
    }
    // Team page (w6/m10: userId + email enrichment) — read-only offline; the
    // member-mutation verbs (invite/change-role/remove) aren't stubbed (a
    // deliberate scope cut, w6/m11/t004: they fall through to the default `{}`
    // response, which the dashboard's mutation hooks treat as a no-op rather
    // than an error). WorkspaceInvites likewise falls through to `{}` — an
    // empty invites list with no error, so the page still renders read-write
    // controls (`canManage` only goes false on an actual GraphQL error).
    case "WorkspaceMembers":
      return {
        workspaceMembers: byOwner(WORKSPACE_MEMBERS, variables.workspaceId),
      };
    // No pending invites seeded — an empty list renders the Team page's real
    // empty state instead of a console warning.
    case "WorkspaceInvites":
      return { workspaceInvites: [] };
    case "ApiKeys":
      return { apiKeys: [] };
    case "RegistryCredentials":
      return { registryCredentials: [] };
    case "NotificationSettings":
      return {
        notificationSettings: {
          __typename: "NotificationSettings",
          deploySucceeded: true,
          deployFailed: true,
        },
      };

    // Settings → Security & Compliance → Audit Log (w4/m14 UI over w4/m10's
    // surface). Admin-scoped in the real API (RelCanManage; a non-admin gets
    // `forbidden` and the whole section hides) — the stub has no auth, so it
    // always answers as an admin, which is what makes the card visible offline.
    // Scoped by ownerId, so switching workspaces MUST swap alpha rows for bravo.
    case "AuditLogs":
      return {
        auditLogs: auditLogsFor({
          ownerId: variables.ownerId,
          cursor: variables.cursor,
          limit: variables.limit,
        }),
      };

    // Managed Postgres (w5/m8) — an interactive in-memory store.
    case "Databases":
      return { databases: byOwner(DATABASES, variables.ownerId) };
    case "Database":
      return { database: DATABASES.find((d) => d.id === variables.id) ?? null };
    case "DatabaseInstanceTypes":
      return { databaseInstanceTypes: DB_INSTANCE_TYPES };
    case "DatabaseConnectionInfo": {
      const d = DATABASES.find((db) => db.id === variables.id);
      if (!d) return { databaseConnectionInfo: null };
      const pw = "s3cr3t_stub_password_not_real_0123456789abcdef";
      const internal = `postgresql://${d.databaseUser}:${pw}@${d.id}-rw.default:5432/${d.databaseName}`;
      return {
        databaseConnectionInfo: {
          __typename: "PostgresConnectionInfo",
          password: pw,
          internalConnectionString: internal,
          externalConnectionString: d.public
            ? `postgresql://${d.databaseUser}:${pw}@${d.externalHost}:5432/${d.databaseName}?sslmode=require&sslnegotiation=direct`
            : "",
          psqlCommand: `PGPASSWORD=${pw} psql -h ${d.id}-rw.default.svc -U ${d.databaseUser} ${d.databaseName}`,
        },
      };
    }
    case "CreateDatabase": {
      const created = makeDatabase({
        name: variables.name,
        plan: variables.plan ?? "free",
        version: variables.version ?? "",
        diskSizeGB: variables.diskSizeGB ?? 0,
        public: Boolean(variables.public),
        status: "creating", // converges to available on the next list poll
        externalHost: variables.public ? `${variables.name}.db.bex.co` : "",
      });
      DATABASES.push(created);
      // Simulate async provisioning: flip to available shortly after.
      setTimeout(() => {
        created.status = "available";
      }, 4000);
      return { createDatabase: created };
    }
    case "DeleteDatabase": {
      const i = DATABASES.findIndex((d) => d.id === variables.id);
      if (i >= 0) DATABASES.splice(i, 1);
      return { deleteDatabase: true };
    }
    // Advanced managed-Postgres surface (w1/m17) + observability (w2/m25) —
    // no bex-stub source for any of these (they read the operator's live
    // CNPG/pg_stat views); safe empties so the detail page's panels render
    // their real "no data yet" states instead of a console warning.
    case "DatabaseRecoveryInfo":
      return { databaseRecoveryInfo: null };
    case "DatabaseUsers":
      return { databaseUsers: [] };
    case "DatabaseIpAllowList":
      return { databaseIpAllowList: [] };
    case "DatabaseProcesses":
      return { databaseProcesses: [] };
    case "DatabaseTopQueries":
      return { databaseTopQueries: [] };
    case "DatabaseSizes":
      return { databaseSizes: null };
    case "DatabaseTableScans":
      return { databaseTableScans: [] };
    case "DatabaseParameterOverrides":
      return { databaseParameterOverrides: [] };
    case "ExecuteDatabaseQuery": {
      const statement = String(variables.sql ?? "").trim();
      if (!statement) throw new Error("sql is required");
      if (variables.allowWrites) {
        return {
          executeDatabaseQuery: {
            __typename: "DatabaseQueryResult",
            columns: [],
            rows: [],
            rowCount: 1,
            truncated: false,
          },
        };
      }
      return {
        executeDatabaseQuery: {
          __typename: "DatabaseQueryResult",
          columns: ["database", "statement", "ok"],
          rows: [
            {
              __typename: "DatabaseQueryRow",
              values: [variables.id, statement, "true"],
            },
          ],
          rowCount: 1,
          truncated: false,
        },
      };
    }
    case "DatastoreMetrics":
      return { datastoreMetrics: [] };
    // Managed Key Value (w5/m12) — an interactive in-memory store, mirroring
    // the Databases stub above (per-id lookup, unknown -> null).
    case "KeyValues":
      return { keyValues: KEY_VALUES };
    case "KeyValue":
      return {
        keyValue: KEY_VALUES.find((k) => k.id === variables.id) ?? null,
      };
    case "KeyValueInstanceTypes":
      return { keyValueInstanceTypes: KV_INSTANCE_TYPES };
    case "KeyValueIpAllowList":
      return { keyValueIpAllowList: [] };
    case "KeyValueConnectionInfo": {
      const k = KEY_VALUES.find((kv) => kv.id === variables.id);
      if (!k) return { keyValueConnectionInfo: null };
      const pw = "s3cr3t_stub_kv_password_not_real_0123456789ab";
      const internal = `redis://:${pw}@${k.id}.default.svc:6379`;
      return {
        keyValueConnectionInfo: {
          __typename: "KeyValueConnectionInfo",
          internalConnectionString: internal,
          externalConnectionString: k.public
            ? `rediss://:${pw}@${k.externalHost}:6379`
            : "",
          cliCommand: `redis-cli -u ${internal}`,
        },
      };
    }
    case "CreateKeyValue": {
      const created = makeKeyValue({
        name: variables.name,
        plan: variables.plan ?? "free",
        version: variables.version ?? "",
        public: Boolean(variables.public),
        status: "creating", // converges to available on the next list/detail poll
        externalHost: variables.public ? `${variables.name}.kv.bex.co` : "",
      });
      KEY_VALUES.push(created);
      // Simulate async provisioning: flip to available shortly after.
      setTimeout(() => {
        created.status = "available";
      }, 4000);
      return { createKeyValue: created };
    }
    case "DeleteKeyValue": {
      const i = KEY_VALUES.findIndex((k) => k.id === variables.id);
      if (i >= 0) KEY_VALUES.splice(i, 1);
      return { deleteKeyValue: true };
    }
    case "SuspendKeyValue": {
      const k = KEY_VALUES.find((kv) => kv.id === variables.id);
      if (!k) return { suspendKeyValue: null };
      k.suspended = "suspended";
      return { suspendKeyValue: k };
    }
    case "ResumeKeyValue": {
      const k = KEY_VALUES.find((kv) => kv.id === variables.id);
      if (!k) return { resumeKeyValue: null };
      k.suspended = "not_suspended";
      return { resumeKeyValue: k };
    }
    // Projects (w1/m31, bex extension) — an interactive in-memory store,
    // mirroring the Databases/KeyValues stubs above. Set*/Create/Rename/Delete
    // mutate PROJECTS in place so the unified dashboard Projects page's
    // grouping + "Move to project" actions are exercised offline.
    case "Projects":
      return { projects: byOwner(PROJECTS, variables.ownerId) };
    case "Project":
      return { project: PROJECTS.find((p) => p.id === variables.id) ?? null };
    case "CreateProject": {
      const created = {
        __typename: "Project",
        id: `prj-local${Date.now().toString(36)}`,
        name: variables.name,
        ownerId: variables.ownerId,
        createdAt: new Date().toISOString(),
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
      };
      PROJECTS.push(created);
      return { createProject: created };
    }
    case "RenameProject": {
      const p = PROJECTS.find((pr) => pr.id === variables.id);
      if (!p) return { renameProject: null };
      p.name = variables.name;
      return { renameProject: p };
    }
    case "DeleteProject": {
      const i = PROJECTS.findIndex((p) => p.id === variables.id);
      if (i >= 0) PROJECTS.splice(i, 1);
      return { deleteProject: variables.id };
    }
    case "SetProjectServices": {
      const p = PROJECTS.find((pr) => pr.id === variables.id);
      if (!p) return { setProjectServices: null };
      p.serviceIds = variables.serviceIds ?? [];
      return { setProjectServices: p };
    }
    case "SetProjectDatabases": {
      const p = PROJECTS.find((pr) => pr.id === variables.id);
      if (!p) return { setProjectDatabases: null };
      p.databaseIds = variables.databaseIds ?? [];
      return { setProjectDatabases: p };
    }
    case "SetProjectKeyValues": {
      const p = PROJECTS.find((pr) => pr.id === variables.id);
      if (!p) return { setProjectKeyValues: null };
      p.keyValueIds = variables.keyValueIds ?? [];
      return { setProjectKeyValues: p };
    }
    // Environments (w1/m32, bex extension) — project-scoped named service
    // subsets, mutated in place so the project page's environments panel is
    // exercised offline (docs/ADR032-environments.md).
    case "Environments":
      return {
        environments: ENVIRONMENTS.filter(
          (e) => e.projectId === variables.projectId,
        ),
      };
    case "Environment":
      return {
        environment: ENVIRONMENTS.find((e) => e.id === variables.id) ?? null,
      };
    case "CreateEnvironment": {
      const project = PROJECTS.find((p) => p.id === variables.projectId);
      const created = {
        __typename: "Environment",
        id: `env-local${Date.now().toString(36)}`,
        projectId: variables.projectId,
        name: variables.name,
        ownerId: project?.ownerId ?? WORKSPACE_DEFAULT,
        createdAt: new Date().toISOString(),
        serviceIds: [],
      };
      ENVIRONMENTS.push(created);
      return { createEnvironment: created };
    }
    case "RenameEnvironment": {
      const e = ENVIRONMENTS.find((env) => env.id === variables.id);
      if (!e) return { renameEnvironment: null };
      e.name = variables.name;
      return { renameEnvironment: e };
    }
    case "DeleteEnvironment": {
      const i = ENVIRONMENTS.findIndex((e) => e.id === variables.id);
      if (i >= 0) ENVIRONMENTS.splice(i, 1);
      return { deleteEnvironment: variables.id };
    }
    case "SetEnvironmentServices": {
      const e = ENVIRONMENTS.find((env) => env.id === variables.id);
      if (!e) return { setEnvironmentServices: null };
      const wanted = variables.serviceIds ?? [];
      e.serviceIds = wanted;
      // Single-column membership: a service is in at most one environment, so
      // assigning it here evicts it from every other environment.
      for (const other of ENVIRONMENTS) {
        if (other.id !== e.id) {
          other.serviceIds = other.serviceIds.filter(
            (s) => !wanted.includes(s),
          );
        }
      }
      // Auto-join the parent project (docs/ADR032): a service "in an
      // environment" is implicitly "in that project".
      const project = PROJECTS.find((p) => p.id === e.projectId);
      if (project) {
        const merged = new Set([...(project.serviceIds ?? []), ...wanted]);
        project.serviceIds = [...merged];
      }
      return { setEnvironmentServices: e };
    }
    case "CustomDomains":
      return { customDomains: domainsFor(variables.id) };
    case "AddCustomDomain": {
      const list = (CUSTOM_DOMAINS_BY_SERVICE[variables.id] ??= []);
      const existing = list.find((d) => d.name === variables.name);
      if (existing) return { addCustomDomain: existing }; // idempotent
      const domain = makeDomain(variables.id, variables.name);
      list.push(domain);
      return { addCustomDomain: domain };
    }
    case "DeleteCustomDomain": {
      const list = CUSTOM_DOMAINS_BY_SERVICE[variables.id] ?? [];
      const i = list.findIndex((d) => d.name === variables.name);
      if (i >= 0) list.splice(i, 1);
      return { deleteCustomDomain: true };
    }
    case "VerifyCustomDomain": {
      // Re-check now: bex verification is automatic, so this simulates the pending
      // domain converging to verified/active on re-check (returns the fresh row).
      const list = CUSTOM_DOMAINS_BY_SERVICE[variables.id] ?? [];
      const d = list.find((dom) => dom.name === variables.name);
      if (!d) return { verifyCustomDomain: null };
      d.verificationStatus = "verified";
      d.serverStatus = "active";
      return { verifyCustomDomain: d };
    }
    // Service events (w5/m7): deploy-event feed per service — a flat
    // ServiceEvent[], no {cursor,events} envelope (each event carries its own
    // cursor instead; see events.graphql's header comment). Cursor pagination
    // is ignored by the stub — always returns the full list newest-first.
    case "ServiceEvents": {
      return { serviceEvents: EVENTS_BY_SERVICE[variables.serviceId] ?? [] };
    }
    // TriggerDeploy: prepend a new in-progress event AND a matching Deploy row
    // (w9/m1/t005), simulate it going live over 5s — the deploy detail page's
    // poll-until-terminal loop is exercisable offline. Note: variables.serviceId,
    // NOT variables.id — this mutation's arg is serviceId (deployMutationArgs'
    // shape), matching Cancel/Rollback below.
    case "TriggerDeploy": {
      const svc = serviceById(variables.serviceId);
      const deploy = makeDeploy({
        id: `dep-stub-${Date.now().toString(36)}`,
        status: "update_in_progress",
        trigger: "api",
        image: svc ? `registry.example.com/${svc.id}:stub` : null,
        // A pre-deploy step runs only when the service has one configured
        // (Settings → preDeployCommand, w1/m33); starts "running" so the deploy
        // page's log panel has a predeploy line to show immediately.
        preDeployStatus: svc?.preDeployCommand ? "running" : "",
      });
      (DEPLOYS_BY_SERVICE[variables.serviceId] ??= []).unshift(deploy);
      (EVENTS_BY_SERVICE[variables.serviceId] ??= []).unshift(
        deployServiceEvent(deploy, {
          __typename: "DeployTrigger",
          firstBuild: false,
          envUpdated: false,
          manual: true,
          deployedByRender: false,
          clearCache: false,
          rollback: false,
        }),
      );
      // Only the Deploy row's own fields need updating — ServiceEvents reads
      // them live via deployServiceEvent's getter, so there's no second copy
      // to remember to flip alongside it.
      setTimeout(() => {
        if (deploy.preDeployStatus) deploy.preDeployStatus = "succeeded";
        deploy.status = "live";
        deploy.finishedAt = new Date().toISOString();
      }, 5000);
      return { triggerDeploy: deploy };
    }
    // CancelDeploy: close the Deploy row canceled — the Events tab picks it
    // up automatically (deployServiceEvent's live getter).
    case "CancelDeploy": {
      const deploy = deployById(variables.serviceId, variables.deployId);
      if (deploy) {
        deploy.status = "canceled";
        deploy.finishedAt = new Date().toISOString();
      }
      return { cancelDeploy: { __typename: "Deploy", id: variables.deployId, status: "canceled" } };
    }
    // RollbackService: prepend a rollback-triggered Deploy row + its event.
    case "RollbackService": {
      const svc = serviceById(variables.serviceId);
      const deploy = makeDeploy({
        id: `dep-rollback-${Date.now().toString(36)}`,
        status: "live",
        trigger: "rollback",
        rollbackOf: variables.deployId,
        image: svc ? `registry.example.com/${svc.id}:rollback` : null,
      });
      (DEPLOYS_BY_SERVICE[variables.serviceId] ??= []).unshift(deploy);
      (EVENTS_BY_SERVICE[variables.serviceId] ??= []).unshift(
        deployServiceEvent(deploy, {
          __typename: "DeployTrigger",
          firstBuild: false,
          envUpdated: false,
          manual: false,
          deployedByRender: false,
          clearCache: false,
          rollback: true,
        }),
      );
      return { rollbackService: deploy };
    }
    // SetHealthCheckPath: persist the new path on the in-memory service object.
    case "SetHealthCheckPath": {
      const svc = serviceById(variables.id);
      if (svc) svc.healthCheckPath = variables.path || "/";
      return {
        setHealthCheckPath: {
          __typename: "Service",
          id: variables.id,
          healthCheckPath: svc?.healthCheckPath ?? variables.path,
        },
      };
    }
    // SetPreDeployCommand (w1/m33): persist the pre-deploy command; empty clears it.
    case "SetPreDeployCommand": {
      const svc = serviceById(variables.id);
      if (svc) svc.preDeployCommand = String(variables.command ?? "").trim() || null;
      return { setPreDeployCommand: { __typename: "Service", id: variables.id, preDeployCommand: svc?.preDeployCommand ?? null, phase: svc?.phase ?? null } };
    }
    // Blueprints (w7/m27) — no seeded blueprints in the dev stub; return an
    // empty list so Apollo's InMemoryCache doesn't log "Missing field" warnings
    // when writing a `{}` fallback for an unhandled operation name.
    case "Blueprints":
      return { blueprints: [] };
    case "Blueprint":
      return { blueprint: null };
    case "ValidateBlueprint":
      return {
        validateBlueprint: {
          __typename: "BlueprintValidationResult",
          valid: true,
          errors: [],
        },
      };
    case "SyncBlueprint":
      return { syncBlueprint: null };
    default:
      return {};
  }
}

const server = createServer((req, res) => {
  cors(req, res);
  const url = new URL(req.url, `http://localhost:${PORT}`);

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  // Kratos settings flow — the /settings page's Ory Elements <Settings> form.
  // Without it, useOryFlow's AJAX createBrowserSettingsFlow 404s and the hook
  // falls back to `bootstrapViaKratos`, a full-page navigation to this very URL
  // — which used to land the browser on the stub's 404 JSON, taking the whole
  // Settings page (Team, API Keys, Security & Compliance) down with it. A
  // minimal browser-type SettingsFlow keeps the user on the page; the form
  // itself is inert offline (no submit handler is stubbed), which is fine —
  // the cards below it are what the offline loop is for.
  if (
    url.pathname === "/self-service/settings/browser" ||
    url.pathname === "/self-service/settings/flows"
  ) {
    const now = Date.now();
    return json(res, 200, {
      id: url.searchParams.get("id") || "local-dev-settings-flow",
      type: "browser",
      expires_at: new Date(now + 3600_000).toISOString(),
      issued_at: new Date(now).toISOString(),
      request_url: `http://localhost:${PORT}${req.url}`,
      state: "show_form",
      identity: {
        id: "local-dev-user",
        schema_id: "default",
        traits: { email: "dev@localhost" },
      },
      ui: {
        action: `http://localhost:${PORT}/self-service/settings?flow=local-dev-settings-flow`,
        method: "POST",
        messages: [],
        nodes: [
          {
            type: "input",
            group: "default",
            attributes: {
              name: "csrf_token",
              type: "hidden",
              value: "local-dev-csrf",
              required: true,
              disabled: false,
              node_type: "input",
            },
            messages: [],
            meta: {},
          },
          {
            type: "input",
            group: "profile",
            attributes: {
              name: "traits.email",
              type: "email",
              value: "dev@localhost",
              required: true,
              disabled: false,
              node_type: "input",
            },
            messages: [],
            meta: {
              label: {
                id: 1070002,
                text: "E-Mail",
                type: "info",
                context: { title: "E-Mail" },
              },
            },
          },
          {
            type: "input",
            group: "profile",
            attributes: {
              name: "method",
              type: "submit",
              value: "profile",
              disabled: false,
              node_type: "input",
            },
            messages: [],
            meta: { label: { id: 1070003, text: "Save", type: "info" } },
          },
        ],
      },
    });
  }

  // Hydra admin consent sessions — the Settings page's "Connected agents"
  // card, via the dashboard's own /api/connected-agents server route (which
  // reads HYDRA_ADMIN_URL; yarn dev:local points it here). One seeded client
  // so list + revoke are exercised offline.
  if (url.pathname === "/admin/oauth2/auth/sessions/consent") {
    if (req.method === "DELETE") {
      const client = url.searchParams.get("client");
      const i = CONSENT_SESSIONS.findIndex(
        (s) => s.consent_request?.client?.client_id === client,
      );
      if (i >= 0) CONSENT_SESSIONS.splice(i, 1);
      res.writeHead(204);
      res.end();
      return;
    }
    return json(res, 200, CONSENT_SESSIONS);
  }

  // Kratos listMySessions — the Settings page's "Active sessions" card
  // (GET /sessions lists the identity's OTHER sessions; the current one comes
  // from whoami). One seeded row so the revoke button has something to act on.
  if (url.pathname === "/sessions" && req.method === "GET") {
    return json(res, 200, [
      {
        id: "local-dev-session-other",
        active: true,
        authenticated_at: "2026-07-10T09:00:00Z",
        devices: [
          {
            ip_address: "203.0.113.7",
            location: "Berlin, DE",
            user_agent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/128.0",
          },
        ],
      },
    ]);
  }

  // Kratos session check — return a fixed active session so the auth guard passes.
  if (url.pathname === "/sessions/whoami") {
    return json(res, 200, {
      id: "local-dev-session",
      active: true,
      identity: {
        id: "local-dev-user",
        traits: { email: "dev@localhost" },
      },
    });
  }

  // Open Deploy Hook trigger (w2/m33): credential is the full secret URL, so
  // no session/API-key check applies in either the real API or this dev stub.
  if (
    url.pathname.startsWith("/v1/deploy-hooks/") &&
    (req.method === "GET" || req.method === "POST")
  ) {
    const hook = [...DEPLOY_HOOKS.entries()].find(([, value]) => {
      const hookURL = new URL(value);
      return hookURL.pathname === url.pathname;
    });
    if (!hook) return json(res, 404, { error: "app not found" });
    return json(res, 200, {
      deploy: { id: `dep-local-hook-${Date.now().toString(36)}` },
    });
  }

  // GraphQL reads.
  if (url.pathname === "/graphql" && req.method === "POST") {
    let body = "";
    req.on("data", (c) => (body += c));
    req.on("end", () => {
      let payload = {};
      try {
        payload = JSON.parse(body || "{}");
      } catch {
        return json(res, 400, { errors: [{ message: "bad JSON" }] });
      }
      const ops = Array.isArray(payload) ? payload : [payload];
      // A resolver throws (e.g. the Hobby-plan-cap refusal, a bad delete
      // confirmation) to simulate bex-api's GraphQL error responses, so the
      // dashboard's inline-error paths are exercisable offline too.
      const results = ops.map((op) => {
        try {
          return { data: resolveGraphQL(op) };
        } catch (err) {
          return { errors: [{ message: err.message }] };
        }
      });
      return json(res, 200, Array.isArray(payload) ? results : results[0]);
    });
    return;
  }

  // SSE live tail — one `data: <renderLog JSON>` frame per new line.
  if (url.pathname === "/v1/logs/subscribe") {
    const type = url.searchParams.get("type");
    const resource = url.searchParams.get("resource") || "";
    res.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    });
    // Request logs have no backend on bex — hold the stream open, emit nothing.
    if (type && type !== "app" && type !== "application") return;
    const text = (url.searchParams.get("text") || "").toLowerCase();
    let seq = 0;
    const timer = setInterval(() => {
      const entry = line(new Date().toISOString(), seq, resource);
      if (!text || entry.message.toLowerCase().includes(text)) {
        res.write(
          `data: ${JSON.stringify(renderLog(entry, seq, resource))}\n\n`,
        );
      }
      seq++;
    }, 1500);
    req.on("close", () => clearInterval(timer));
    return;
  }

  json(res, 404, { error: "not found" });
});

server.listen(PORT, () => {
  console.log(`local-bex (dev stub) listening on http://localhost:${PORT}`);
  console.log(`  GraphQL:  POST http://localhost:${PORT}/graphql`);
  console.log(`  Live tail: GET http://localhost:${PORT}/v1/logs/subscribe`);
  console.log(`  Kratos:   GET  http://localhost:${PORT}/sessions/whoami`);
  console.log(`  Kratos:   GET  http://localhost:${PORT}/sessions`);
  console.log(`  Hydra:    GET  http://localhost:${PORT}/admin/oauth2/auth/sessions/consent`);
});

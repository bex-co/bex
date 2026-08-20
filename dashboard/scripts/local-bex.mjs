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

// API keys are empty by default but create/revoke are interactive so the
// account-settings mint-once flow can be browser-verified offline. The stored
// rows intentionally exclude the secret; only CreateApiKey's immediate
// response carries it, matching bex-api and Render's one-time display rule.
const API_KEYS = [];

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

// Fields every dashboard Service selection expects, with the neutral value.
// Each fixture spreads this FIRST and overrides what makes it interesting, so
// a newly selected schema field lands here once instead of per fixture (a
// missing selected field makes Apollo drop the whole cache write — the
// docs-site regression this replaced, w5/m48).
const SERVICE_DEFAULTS = {
  __typename: "Service",
  displayName: null,
  suspended: null,
  sshAddress: null,
  schedule: null,
  command: null,
  runs: [],
  healthCheckPath: null,
  notifyOnFail: "default",
  notificationsToSend: "default",
  maxShutdownDelaySeconds: null,
  preDeployCommand: null,
  idleTTLSeconds: 0,
  maintenanceMode: null,
  lastSuccessfulRunAt: null,
  renderSubdomainPolicy: null,
  repo: null,
  branch: null,
  rootDir: null,
  buildFilter: null,
  runtime: null,
  region: null,
  builder: null,
  buildCommand: null,
  startCommand: null,
  dockerfilePath: null,
  registryCredentialId: null,
  autoDeploy: null,
  publishPath: null,
  routes: [],
  headers: [],
  ipAllowList: [],
  ipAllowListEntries: [],
  ownerId: WORKSPACE_DEFAULT,
};

// One sample App, Render-shaped (matches the dashboard's Service selection set).
// ownerId scopes it to the default workspace — a stub-only field, harmless if
// it leaks into a response the dashboard didn't select it in.
const SERVICE = {
  ...SERVICE_DEFAULTS,
  id: "eden-cms-v2",
  name: "eden-cms-v2",
  slug: "eden-cms-v2",
  type: "web_service",
  dashboardUrl: "http://localhost:5173/services/eden-cms-v2",
  url: "https://eden-cms-v2.onbex.co",
  createdAt: "2026-06-01T09:00:00Z",
  updatedAt: "2026-07-16T18:30:00Z",
  phase: "Running",
  replicas: 2,
  revision: "a1b2c3d",
  plan: "starter",
  healthCheckPath: "/healthz",
  maxShutdownDelaySeconds: 30,
  repo: "https://github.com/acme-corp/eden-cms-v2",
  branch: "main",
  rootDir: "",
  runtime: "docker",
  region: "fsn1",
  builder: "dockerfile",
  startCommand: "node server.js",
  dockerfilePath: "Dockerfile",
  autoDeploy: true,
};

// Two more service types (w1/m15): a background_worker (no HTTP port/URL) and a
// cron_job (schedule + run history). Seeds so the type badge, the worker's
// no-URL detail, and the cron's schedule + recent-runs section all render.
const WORKER = {
  ...SERVICE_DEFAULTS,
  id: "email-worker",
  name: "email-worker",
  slug: "email-worker",
  type: "background_worker",
  dashboardUrl: "http://localhost:5173/services/email-worker",
  url: null,
  createdAt: "2026-06-05T11:30:00Z",
  updatedAt: "2026-07-15T12:15:00Z",
  phase: "Running",
  replicas: 1,
  revision: "9f8e7d6",
  plan: "starter",
  maxShutdownDelaySeconds: 45,
  runtime: "node",
  region: "fsn1",
};

const CRON = {
  ...SERVICE_DEFAULTS,
  id: "nightly-report",
  name: "nightly-report",
  slug: "nightly-report",
  type: "cron_job",
  dashboardUrl: "http://localhost:5173/services/nightly-report",
  url: null,
  createdAt: "2026-06-10T08:00:00Z",
  updatedAt: null,
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
};

// A static_site (w5/m48): exercises the type-gated IA — no Logs/Shell/Scaling/
// Plan, dedicated Redirects/Rewrites + Headers pages, no Instances fact, no
// Instance Type row — and the from-creation URL (bex-api derives it before the
// first successful publish; the stub mirrors the derived shape).
const STATIC = {
  ...SERVICE_DEFAULTS,
  id: "docs-site",
  name: "docs-site",
  slug: "docs-site",
  type: "static_site",
  dashboardUrl: "http://localhost:5173/services/docs-site",
  url: "https://docs-site.onbex.co",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-17T09:00:00Z",
  phase: "Running",
  replicas: null,
  revision: "d4c3b2a",
  plan: "free",
  renderSubdomainPolicy: "enabled",
  repo: "https://github.com/acme-corp/docs-site",
  branch: "main",
  rootDir: "site",
  runtime: "node",
  region: "fsn1",
  buildCommand: "npm run build",
  autoDeploy: true,
  publishPath: "dist",
  routes: [
    {
      __typename: "StaticRoute",
      type: "rewrite",
      source: "/*",
      destination: "/index.html",
    },
  ],
  headers: [
    {
      __typename: "StaticHeader",
      path: "/*",
      name: "X-Frame-Options",
      value: "DENY",
    },
  ],
};

const SERVICES = [SERVICE, WORKER, CRON, STATIC];

// Environment-page fixtures (w5/m44): values stay in memory and list queries
// return keys/names only, matching bex-api's sensitive-read boundary. Set
// LOCAL_BEX_FAIL_ENV_SAVE_ONCE=1 to make the next coherent save fail, or
// LOCAL_BEX_FAIL_DEPLOY_ONCE=1 to exercise save-success/deploy-failure recovery.
const ENV_BY_SERVICE = new Map([
  [
    SERVICE.id,
    new Map([
      ["APP_MODE", "production"],
      ["DATABASE_URL", "postgres://local:local@db.local/eden"],
    ]),
  ],
  [WORKER.id, new Map([["QUEUE_NAME", "outbound-email"]])],
  [CRON.id, new Map()],
]);
const SECRET_FILES_BY_SERVICE = new Map([
  [SERVICE.id, new Map([["service-account.json", '{"project":"local"}\n']])],
  [WORKER.id, new Map()],
  [CRON.id, new Map()],
]);
let failEnvironmentSaveOnce = process.env.LOCAL_BEX_FAIL_ENV_SAVE_ONCE === "1";
let failDeployOnce = process.env.LOCAL_BEX_FAIL_DEPLOY_ONCE === "1";

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
// Wall-clock-relative timestamps (computed at boot) so the Metrics tab's live
// window — capped at "Last day" — always contains some events; the fixed-date
// rows below age out of it and only show on the Events tab.
const minutesAgo = (m) => new Date(Date.now() - m * 60_000).toISOString();

// Synthetic per-metric waveforms for the Metrics tab (metrics.graphql's
// MetricsQueryInput → MetricSeries[]), so its charts — and the chart event
// markers overlaid on them — render offline. Deterministic in wall-clock time:
// a poll tick extends the series instead of redrawing a new random shape.
function syntheticMetrics(query) {
  const name = query?.name ?? "";
  const resource =
    (query?.filters ?? []).find((f) => f?.field === "RESOURCE")?.values?.[0] ??
    "app";
  const end = query?.end ? Date.parse(query.end) : Date.now();
  const start = query?.start ? Date.parse(query.start) : end - 3_600_000;
  const stepMs = (query?.resolution || 60) * 1000;
  const wobble = (t, periodSeconds, amp) =>
    amp * Math.sin((t / 1000 / periodSeconds) * 2 * Math.PI);
  const series = (unit, valueAt) => {
    const values = [];
    for (let t = start; t <= end; t += stepMs) {
      values.push({
        __typename: "MetricValue",
        time: new Date(t).toISOString(),
        value: valueAt(t),
      });
    }
    return [
      {
        __typename: "MetricSeries",
        unit,
        labels: [
          {
            __typename: "MetricLabel",
            field: "instance",
            value: `${resource}-stub-1`,
          },
        ],
        values,
        parameters: null,
      },
    ];
  };
  switch (name) {
    case "CPU":
      return series(
        "cpu",
        (t) => 0.08 + wobble(t, 900, 0.03) + wobble(t, 137, 0.01),
      );
    case "CPU_LIMIT":
      return series("cpu", () => 0.5);
    case "MEMORY":
      return series(
        "bytes",
        (t) => 200e6 + wobble(t, 1200, 30e6) + wobble(t, 173, 5e6),
      );
    case "MEMORY_LIMIT":
      return series("bytes", () => 512 * 1024 * 1024);
    case "INSTANCES":
      return series("count", () => 1);
    case "HTTP_REQUESTS":
      return series("count", (t) =>
        Math.max(0, Math.round(40 + wobble(t, 600, 15) + wobble(t, 97, 6))),
      );
    case "HTTP_LATENCY":
      return series("seconds", (t) => 0.12 + Math.abs(wobble(t, 700, 0.05)));
    case "BANDWIDTH":
      return series("bytes", (t) => 2e6 + Math.abs(wobble(t, 800, 1.2e6)));
    default:
      // CPU_TARGET / MEMORY_TARGET etc.: autoscaling off — no series.
      return [];
  }
}

const EVENTS_BY_SERVICE = {
  "eden-cms-v2": [
    // Recent events in the real wire vocabulary (deploy_started/deploy_ended
    // + store.RenderDeployStatus's succeeded|failed), so the Metrics tab's
    // chart event markers and timeline render offline.
    {
      __typename: "ServiceEvent",
      id: "evt-restart-002",
      type: "server_restarted",
      timestamp: minutesAgo(8),
      cursor: "evt-restart-002",
      details: {
        __typename: "ServiceEventDetails",
        deployId: null,
        deployStatus: null,
        preDeployStatus: null,
        actor: "dev@localhost",
        triggeredByUser: "dev@localhost",
        image: null,
        commitId: null,
        commitMessage: null,
        startedAt: null,
        finishedAt: null,
        trigger: null,
      },
    },
    {
      __typename: "ServiceEvent",
      id: "evt-ended-002",
      type: "deploy_ended",
      timestamp: minutesAgo(22),
      cursor: "evt-ended-002",
      details: {
        __typename: "ServiceEventDetails",
        deployId: "dep-live-001",
        deployStatus: "succeeded",
        preDeployStatus: "succeeded",
        actor: "dev@localhost",
        triggeredByUser: "dev@localhost",
        image: null,
        commitId: "a1318dbcafe0123",
        commitMessage: "feat: stub deploy",
        startedAt: null,
        finishedAt: null,
        trigger: null,
      },
    },
    {
      __typename: "ServiceEvent",
      id: "evt-start-002",
      type: "deploy_started",
      timestamp: minutesAgo(26),
      cursor: "evt-start-002",
      details: {
        __typename: "ServiceEventDetails",
        deployId: "dep-live-001",
        deployStatus: null,
        preDeployStatus: null,
        actor: "dev@localhost",
        triggeredByUser: "dev@localhost",
        image: null,
        commitId: null,
        commitMessage: null,
        startedAt: null,
        finishedAt: null,
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
      id: "evt-ended-001f",
      type: "deploy_ended",
      timestamp: minutesAgo(45),
      cursor: "evt-ended-001f",
      details: {
        __typename: "ServiceEventDetails",
        deployId: "dep-failed-000",
        deployStatus: "failed",
        preDeployStatus: "failed",
        actor: "github",
        triggeredByUser: null,
        image: null,
        commitId: null,
        commitMessage: null,
        startedAt: null,
        finishedAt: null,
        trigger: null,
      },
    },
    {
      __typename: "ServiceEvent",
      id: "evt-live-001",
      type: "deploy",
      timestamp: "2026-07-11T14:30:00Z",
      cursor: "dep-live-001",
      details: {
        __typename: "ServiceEventDetails",
        deployId: "dep-live-001",
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
      id: "evt-failed-000",
      type: "deploy",
      timestamp: "2026-07-11T13:00:00Z",
      cursor: "dep-failed-000",
      details: {
        __typename: "ServiceEventDetails",
        deployId: "dep-failed-000",
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
      commitMessage:
        "feat(cms): lazy-load the asset picker\n\nCuts editor TTI by ~400ms.",
      commitCreatedAt: null,
      createdAt: "2026-07-11T14:30:00Z",
      updatedAt: "2026-07-11T14:31:40Z",
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
      commitCreatedAt: null,
      createdAt: "2026-07-11T13:00:00Z",
      updatedAt: "2026-07-11T13:01:12Z",
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
function makeDeploy({
  id,
  status,
  trigger,
  rollbackOf = "",
  image,
  preDeployStatus = "",
  commitId = "",
  commitMessage = "",
}) {
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
    commitCreatedAt: null,
    updatedAt: createdAt,
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
    id: `evt-${deploy.id.slice(4)}`,
    type: "deploy",
    timestamp: deploy.createdAt,
    cursor: deploy.id,
    get details() {
      return {
        __typename: "ServiceEventDetails",
        deployId: deploy.id,
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
    redirectForName: null,
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
    id: "dpg-c185th5c2rvvnhbfiltg",
    name,
    plan: "basic-1gb",
    version: "16",
    status: "available",
    databaseName: dbn,
    databaseUser: `${dbn}_user`,
    diskSizeGB: 5,
    diskAutoscalingEnabled: false,
    highAvailabilityEnabled: false,
    readReplicas: [],
    suspended: "not_suspended",
    createdAt: "2026-06-20T10:00:00Z",
    updatedAt: "2026-07-14T20:00:00Z",
    region: "fsn1",
    externalHost: "orders-db.db.bex.co",
    public: true,
    poolerEnabled: false,
    backupsEnabled: false,
    ipAllowList: [],
    ipAllowListEntries: [],
    projectId: null,
    environmentId: null,
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
    updatedAt: "2026-07-13T16:45:00Z",
    region: "fsn1",
    externalHost: "sessions-cache.kv.bex.co",
    public: true,
    ipAllowListEntries: [],
    projectId: null,
    environmentId: null,
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
    updatedAt: null,
    region: null,
  }),
];

// Projects (w1/m31, bex extension — an interactive in-memory store, mirroring
// Databases/KeyValues above). One seeded project spanning all three resource
// kinds (a service + a database) so the unified dashboard Projects page's
// merged grouping is visibly exercised offline. The resources span two
// Environments below, while "nightly-report" and "rate-limiter" remain in the
// Project but unassigned so the selected-Environment and Unassigned views can
// both be exercised in a real browser.
const PROJECTS = [
  {
    __typename: "Project",
    id: "prj-local0001",
    name: "storefront",
    ownerId: WORKSPACE_DEFAULT,
    createdAt: "2026-06-25T09:00:00Z",
    serviceIds: ["eden-cms-v2", "email-worker", "nightly-report"],
    databaseIds: ["dpg-c185th5c2rvvnhbfiltg"],
    keyValueIds: ["sessions-cache", "rate-limiter"],
  },
];

// Environments (w1/m32, bex extension — docs/ADR032-environments.md): named
// subsets of all supported Project resources. Two populated rows make the
// selected-Environment table, metadata, contextual creation, and mixed-kind
// bulk Move flow browser-verifiable offline. The SetEnvironment* mutations
// auto-join assigned resources to the parent Project and evict them from every
// other Environment, matching the real store's single-column membership.
const ENVIRONMENTS = [
  {
    __typename: "Environment",
    id: "env-localproduction",
    projectId: "prj-local0001",
    name: "Production",
    ownerId: WORKSPACE_DEFAULT,
    createdAt: "2026-06-25T10:00:00Z",
    serviceIds: ["eden-cms-v2"],
    databaseIds: ["dpg-c185th5c2rvvnhbfiltg"],
    keyValueIds: ["sessions-cache"],
    envGroupIds: ["evg-localproduction"],
    protectedStatus: "protected",
    networkIsolationEnabled: true,
    ipAllowList: [],
    ipAllowListEntries: [],
  },
  {
    __typename: "Environment",
    id: "env-localstaging",
    projectId: "prj-local0001",
    name: "Staging",
    ownerId: WORKSPACE_DEFAULT,
    createdAt: "2026-06-25T10:05:00Z",
    serviceIds: ["email-worker"],
    databaseIds: [],
    keyValueIds: [],
    envGroupIds: [],
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowList: [],
    ipAllowListEntries: [],
  },
];

const ENV_GROUPS = [
  {
    __typename: "EnvGroup",
    id: "evg-localproduction",
    name: "production-shared",
    ownerId: WORKSPACE_DEFAULT,
    createdAt: "2026-06-25T10:10:00Z",
    updatedAt: "2026-07-12T09:30:00Z",
    serviceLinks: ["eden-cms-v2"],
    envVars: [{ __typename: "EnvVar", key: "APP_MODE" }],
    secretFiles: [],
  },
];

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
    targetName: null,
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
  // Two ANSI-colorized lines shaped like real BuildKit/Tailwind build output,
  // so the local viewer exercises the escape-interpreting render path
  // (dashboard/src/features/logs/lib/ansi.ts) the way production build logs do.
  "#11 94.34 \u001b[2m│\u001b[22m     min-height: var(--feed-reserve-*);",
  "#11 94.34 \u001b[2m┆\u001b[22m                    \u001b[33m\u001b[2m^--\u001b[22m Unexpected token Delim('*')\u001b[39m",
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
    "Authorization, Content-Type, X-Session-Token, X-Bex-Agent-Ticket",
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

/**
 * Apply the real Environment store's full-replace, single-membership contract
 * to one resource kind. Assigning ids to the target removes them from every
 * sibling Environment. Project-owned kinds are also auto-joined to the parent.
 */
function setEnvironmentMembers({
  environmentId,
  environmentField,
  projectField,
  ids,
}) {
  const environment = ENVIRONMENTS.find((e) => e.id === environmentId);
  if (!environment) return null;

  const wanted = [...new Set(ids ?? [])];
  environment[environmentField] = wanted;
  for (const sibling of ENVIRONMENTS) {
    if (sibling.id !== environment.id) {
      sibling[environmentField] = (sibling[environmentField] ?? []).filter(
        (id) => !wanted.includes(id),
      );
    }
  }

  const project = PROJECTS.find((p) => p.id === environment.projectId);
  if (project && projectField) {
    project[projectField] = [
      ...new Set([...(project[projectField] ?? []), ...wanted]),
    ];
  }
  return environment;
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
      const createdAt = new Date().toISOString();
      const svc = {
        __typename: "Service",
        id: variables.name,
        name: variables.name,
        slug: variables.name,
        displayName: null,
        type: variables.image ? "web_service" : "web_service",
        suspended: null,
        dashboardUrl: `http://localhost:5173/services/${variables.name}`,
        url: `https://${variables.name}.onbex.co`,
        createdAt,
        updatedAt: createdAt,
        phase: "Pending",
        replicas: 1,
        revision: null,
        plan: variables.plan ?? "free",
        schedule: null,
        command: null,
        runs: [],
        healthCheckPath: null,
        notifyOnFail: "default",
        notificationsToSend: "default",
        maxShutdownDelaySeconds: 30,
        preDeployCommand: null,
        idleTTLSeconds: 0,
        lastSuccessfulRunAt: null,
        renderSubdomainPolicy: null,
        ownerId: WORKSPACE_DEFAULT,
        repo: variables.repo ?? null,
        branch: variables.branch ?? null,
        rootDir: variables.rootDir ?? null,
        runtime: variables.runtime ?? null,
        region: "fsn1",
        builder: variables.runtime === "docker" ? "dockerfile" : "native",
        buildCommand: variables.buildCommand ?? null,
        startCommand: variables.startCommand ?? null,
        dockerfilePath: variables.dockerfilePath ?? null,
        autoDeploy: variables.autoDeploy ?? true,
        buildFilter: null,
        publishPath: null,
        routes: [],
        headers: [],
        image: variables.image ?? null,
      };
      SERVICES.push(svc);
      if (variables.environmentId) {
        const current = ENVIRONMENTS.find(
          (environment) => environment.id === variables.environmentId,
        );
        const environment = setEnvironmentMembers({
          environmentId: variables.environmentId,
          environmentField: "serviceIds",
          projectField: "serviceIds",
          ids: [...(current?.serviceIds ?? []), svc.id],
        });
        svc.environmentId = environment?.id ?? null;
        svc.projectId = environment?.projectId ?? null;
      } else {
        svc.environmentId = null;
        svc.projectId = null;
      }
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
    case "ServiceNameAvailable": {
      const available = !SERVICES.some(
        (service) => service.name === variables.name,
      );
      return {
        serviceNameAvailable: {
          __typename: "NameAvailability",
          available,
          suggestion: available ? null : `${variables.name}-1`,
        },
      };
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
    case "SetStartCommand": {
      const svc = serviceById(variables.id);
      if (!svc) throw new Error("not found");
      svc.startCommand = String(variables.command ?? "").trim();
      return { setStartCommand: svc };
    }
    case "SetDockerfilePath": {
      const svc = serviceById(variables.id);
      if (!svc) throw new Error("not found");
      const next = String(variables.dockerfilePath ?? "").trim();
      if (next.startsWith("/") || next.split("/").includes("..")) {
        throw new Error(
          "dockerfilePath must be a relative path with no '..' components",
        );
      }
      svc.dockerfilePath = next;
      return { setDockerfilePath: svc };
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
        return {
          logs: lines.filter((l) => inWindow(l.timestamp)).slice(-limit),
        };
      }
      if (type && type !== "app" && type !== "application") return { logs: [] };
      const isDatabase = DATABASES.some((database) => database.id === resource);
      let logs = history(60, resource)
        .map((entry, index) =>
          isDatabase
            ? {
                ...entry,
                message: [
                  "checkpoint complete: wrote 128 buffers",
                  "duration: 2451.813 ms  statement: SELECT * FROM orders",
                  "automatic vacuum of table orders: index scans: 1",
                  "connection authorized: user=orders_db_user database=orders_db",
                ][index % 4],
                type: "postgres",
                level: null,
              }
            : entry,
        )
        .filter((l) => inWindow(l.timestamp));
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
      if (statuses.length > 0)
        rows = rows.filter((d) => statuses.includes(d.status));
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
      return { metrics: syntheticMetrics(variables.query ?? {}) };
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
      // Synthetic charge tree matching the price sheet (pricing.yaml): starter
      // compute $4.90/mo per 2,628,000 s; egress $0.015/GiB; build $0.0035/min.
      // The dashboard reads `estimatedCost.resources` — per-resource cost plus
      // the charge lines behind it — so the stub builds the same shape the
      // backend's pricing.Estimate does, rather than a flat meter list.
      const RATES = {
        instance_seconds: { starter: 0.000001864535768645, free: 0 },
        egress_bytes: 0.000000000013969838619232,
        build_seconds: 0.000058333333333333,
      };
      const UNITS = {
        instance_seconds: { unit: "hr", per: 3600 },
        egress_bytes: { unit: "GB", per: 1073741824 },
        build_seconds: { unit: "min", per: 60 },
      };
      // Match the backend's formatters: rates carry enough significant digits
      // that rate x quantity still equals the cost printed beside them.
      const fmt = (v, sig, min) => {
        if (v === 0) return "0";
        let decimals = min;
        const exp = Math.floor(Math.log10(Math.abs(v)));
        if (exp < 0) decimals = Math.max(decimals, sig - 1 - exp);
        let out = v.toFixed(decimals);
        while (decimals > min && out.endsWith("0")) out = v.toFixed(--decimals);
        return out;
      };
      const SHAPE = [
        {
          serviceId: "srv-edencms0001",
          serviceName: "eden-cms-v2",
          resourceKind: "service",
          rows: [
            ["instance_seconds", "starter", 432000],
            ["egress_bytes", "", 524288000],
            ["build_seconds", "", 1800],
          ],
        },
        {
          serviceId: "srv-emailwrk0001",
          serviceName: "email-worker",
          resourceKind: "service",
          rows: [
            ["instance_seconds", "starter", 216000],
            ["build_seconds", "", 900],
          ],
        },
        {
          serviceId: "srv-nightly0001",
          serviceName: "nightly-report",
          resourceKind: "service",
          rows: [
            ["instance_seconds", "free", 3600],
            ["build_seconds", "", 300],
          ],
        },
        {
          serviceId: "dpg-primarydb01",
          serviceName: "primary-db",
          resourceKind: "postgres",
          rows: [
            ["instance_seconds", "basic-256mb", 432000],
            ["storage_gb_seconds", "", 4320000],
          ],
        },
      ];
      let totalRaw = 0;
      const resources = SHAPE.map((res) => {
        let resRaw = 0;
        const charges = res.rows.map(([kind, tier, base]) => {
          const qty = Math.round(base * scale);
          const rate =
            kind === "instance_seconds"
              ? res.resourceKind === "postgres"
                ? 0.000005327245053272
                : (RATES.instance_seconds[tier] ?? 0)
              : kind === "storage_gb_seconds"
                ? 0.000000079908675799
                : RATES[kind];
          const cost = rate * qty;
          resRaw += cost;
          const u = UNITS[kind] ?? { unit: "GB-mo", per: 2628000 };
          return {
            __typename: "ChargeLine",
            kind,
            tier,
            unit: u.unit,
            rateUsd: fmt(rate * u.per, 4, 2),
            quantity: fmt(qty / u.per, 3, 2),
            costUsd: cost.toFixed(2),
          };
        });
        totalRaw += resRaw;
        return {
          __typename: "ResourceEstimate",
          serviceId: res.serviceId,
          serviceName: res.serviceName,
          resourceKind: res.resourceKind,
          costUsd: resRaw.toFixed(2),
          charges,
        };
      });
      return {
        usage: {
          __typename: "UsageSummary",
          workspaceId: "local-workspace",
          period,
          estimatedCost: {
            __typename: "EstimatedCost",
            totalUsd: totalRaw.toFixed(2),
            resources,
          },
          // No Stripe contract in the dev stub: estimate-only, like a
          // workspace that has never onboarded a payment method.
          billing: null,
        },
      };
    }
    // Billing onboarding state. The stub has no Stripe, but returning a safe
    // empty here renders the billing page's payment card as a hard error, so
    // give it a coherent "customer exists, card on file, nothing to fix"
    // shape — enough to review the page locally.
    case "BillingReadiness":
      return {
        workspaceBillingReadiness: {
          __typename: "WorkspaceBillingReadiness",
          workspaceId: "local-workspace",
          mode: "test",
          customerReady: true,
          subscriptionReady: true,
          paymentMethodReady: true,
          paymentMethodBrand: "visa",
          paymentMethodLast4: "4242",
          lifecycle: {
            __typename: "BillingLifecycle",
            status: "healthy",
            reason: "",
            graceDeadline: "",
            enforcementOwned: false,
            recoveryPending: false,
            allowedActions: ["update_payment_method", "open_portal"],
            updatedAt: "",
          },
          tax: {
            __typename: "BillingTaxReadiness",
            configured: false,
            enabled: false,
            reason: "product_tax_not_configured",
            productTaxCode: "",
            taxBehavior: "",
            registrationCount: 0,
          },
        },
      };
    case "WorkspaceLimits":
      return {
        workspaceLimits: {
          __typename: "WorkspaceLimits",
          services: { __typename: "ResourceCap", used: 3, limit: 25 },
          postgres: { __typename: "ResourceCap", used: 1, limit: 2 },
          keyValues: { __typename: "ResourceCap", used: 2, limit: 2 },
        },
      };
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
        envVars: [
          ...(ENV_BY_SERVICE.get(variables.serviceId) ?? new Map()).keys(),
        ]
          .sort()
          .map((key) => ({
            __typename: "EnvVarWithCursor",
            envVar: { __typename: "EnvVarListValue", id: key, key },
            cursor: key,
          })),
      };
    case "EnvVarValue": {
      const value = ENV_BY_SERVICE.get(variables.id)?.get(variables.key);
      return {
        service:
          value === undefined
            ? null
            : {
                __typename: "Service",
                id: variables.id,
                envVar: {
                  __typename: "EnvVar",
                  id: variables.key,
                  key: variables.key,
                  value,
                },
              },
      };
    }
    case "SecretFileNames":
      return {
        secretFiles: [
          ...(
            SECRET_FILES_BY_SERVICE.get(variables.serviceId) ?? new Map()
          ).keys(),
        ]
          .sort()
          .map((name) => ({
            __typename: "SecretFileWithCursor",
            secretFile: { __typename: "SecretFileListValue", id: name, name },
            cursor: name,
          })),
      };
    case "SecretFileContent": {
      const content = SECRET_FILES_BY_SERVICE.get(variables.id)?.get(
        variables.name,
      );
      return {
        service:
          content === undefined
            ? null
            : {
                __typename: "Service",
                id: variables.id,
                secretFile: {
                  __typename: "SecretFile",
                  id: variables.name,
                  name: variables.name,
                  content,
                },
              },
      };
    }
    case "PatchServiceEnvironment": {
      if (failEnvironmentSaveOnce) {
        failEnvironmentSaveOnce = false;
        throw new Error("local-bex injected environment save failure");
      }
      const env = ENV_BY_SERVICE.get(variables.serviceId) ?? new Map();
      const secretFiles =
        SECRET_FILES_BY_SERVICE.get(variables.serviceId) ?? new Map();
      for (const patch of variables.envVars ?? []) {
        if (patch.fromKey) {
          if (!env.has(patch.fromKey)) throw new Error("not found");
          const value = env.get(patch.fromKey);
          env.delete(patch.fromKey);
          env.set(patch.key, value);
        } else if (patch.delete) env.delete(patch.key);
        else env.set(patch.key, patch.value ?? "");
      }
      for (const patch of variables.secretFiles ?? []) {
        if (patch.fromName) {
          if (!secretFiles.has(patch.fromName)) throw new Error("not found");
          const content = secretFiles.get(patch.fromName);
          secretFiles.delete(patch.fromName);
          secretFiles.set(patch.name, content);
        } else if (patch.delete) secretFiles.delete(patch.name);
        else secretFiles.set(patch.name, patch.content ?? "");
      }
      ENV_BY_SERVICE.set(variables.serviceId, env);
      SECRET_FILES_BY_SERVICE.set(variables.serviceId, secretFiles);
      return {
        patchServiceEnvironment: {
          __typename: "EnvironmentPatchResult",
          envVarKeys: [...env.keys()].sort(),
          secretFileNames: [...secretFiles.keys()].sort(),
          rolledOut: variables.saveMode === "deploy",
        },
      };
    }
    case "EnvGroups":
      return { envGroups: byOwner(ENV_GROUPS, variables.ownerId) };
    case "CreateEnvGroup": {
      const now = new Date().toISOString();
      const group = {
        __typename: "EnvGroup",
        id: `evg-local${Date.now().toString(36)}`,
        name: variables.name,
        ownerId: variables.ownerId ?? WORKSPACE_DEFAULT,
        createdAt: now,
        updatedAt: now,
        serviceLinks: [...(variables.serviceIds ?? [])],
        envVars: (variables.envVars ?? []).map(({ key }) => ({
          __typename: "EnvVar",
          key,
        })),
        secretFiles: (variables.secretFiles ?? []).map(({ name }) => ({
          __typename: "SecretFile",
          name,
        })),
      };
      ENV_GROUPS.push(group);
      return { createEnvGroup: group };
    }
    case "LinkEnvGroup": {
      const group = ENV_GROUPS.find(
        (candidate) => candidate.id === variables.id,
      );
      if (!group) throw new Error("not found");
      if (!group.serviceLinks.includes(variables.serviceId)) {
        group.serviceLinks.push(variables.serviceId);
        group.updatedAt = new Date().toISOString();
      }
      return { linkEnvGroup: true };
    }
    case "UnlinkEnvGroup": {
      const group = ENV_GROUPS.find(
        (candidate) => candidate.id === variables.id,
      );
      if (!group) throw new Error("not found");
      group.serviceLinks = group.serviceLinks.filter(
        (serviceId) => serviceId !== variables.serviceId,
      );
      group.updatedAt = new Date().toISOString();
      return { unlinkEnvGroup: true };
    }
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
      return { apiKeys: byOwner(API_KEYS, variables.ownerId) };
    case "SSHKeys":
      return { sshKeys: [] };
    case "CreateApiKey": {
      const created = {
        __typename: "ApiKey",
        id: `key-local${Date.now().toString(36)}`,
        name: variables.name,
        ownerId: variables.ownerId ?? WORKSPACE_DEFAULT,
        createdAt: new Date().toISOString(),
        createdBy: "owner@acme-hq.example",
        lastUsedAt: null,
      };
      API_KEYS.push(created);
      return {
        createApiKey: {
          ...created,
          secret: `bex_local_${Date.now().toString(36)}_shown_once`,
        },
      };
    }
    case "RevokeApiKey": {
      const index = API_KEYS.findIndex(
        (key) =>
          key.id === variables.id &&
          key.ownerId === (variables.ownerId ?? WORKSPACE_DEFAULT),
      );
      if (index >= 0) API_KEYS.splice(index, 1);
      return { revokeApiKey: index >= 0 };
    }
    case "RegistryCredentials":
      return { registryCredentials: [] };
    case "NotificationSettings":
      return {
        notificationSettings: {
          __typename: "NotificationSettings",
          deployStarted: true,
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
            ? `postgresql://${d.databaseUser}:${pw}@${d.externalHost}:5432/${d.databaseName}?sslmode=verify-full`
            : "",
          psqlCommand: `PGPASSWORD=${pw} psql -h ${d.id}-rw.default.svc -U ${d.databaseUser} ${d.databaseName}`,
        },
      };
    }
    case "CreateDatabase": {
      const created = makeDatabase({
        id: `dpg-local${Date.now().toString(36)}`,
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
    case "UpdateDatabaseDiskAutoscaling": {
      const database = DATABASES.find((d) => d.id === variables.id);
      if (!database) throw new Error("not found");
      database.diskAutoscalingEnabled = Boolean(variables.enabled);
      return { updateDatabaseDiskAutoscaling: database };
    }
    // Advanced managed-Postgres surface (w1/m17) + observability (w2/m25) —
    // no bex-stub source for any of these (they read the operator's live
    // CNPG/pg_stat views); safe empties so the detail page's panels render
    // their real "no data yet" states instead of a console warning.
    case "DatabaseRecoveryInfo":
      return { databaseRecoveryInfo: null };
    case "DatabaseUsers":
      return { databaseUsers: [] };
    case "DatabaseIpAllowList": {
      const database = DATABASES.find((entry) => entry.id === variables.id);
      return {
        database: database
          ? {
              __typename: "Database",
              id: database.id,
              ipAllowListEntries: database.ipAllowListEntries ?? [],
            }
          : null,
      };
    }
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
    case "KeyValueIpAllowList": {
      const keyValue = KEY_VALUES.find((entry) => entry.id === variables.id);
      return {
        keyValue: keyValue
          ? {
              __typename: "KeyValue",
              id: keyValue.id,
              ipAllowListEntries: keyValue.ipAllowListEntries ?? [],
            }
          : null,
      };
    }
    case "KeyValueConnectionInfo": {
      const k = KEY_VALUES.find((kv) => kv.id === variables.id);
      if (!k) return { keyValueConnectionInfo: null };
      const pw = "s3cr3t_stub_kv_password_not_real_0123456789ab";
      // Explicit "default" user, not the empty-username redis://:<password>@
      // shorthand: valkey-cli 8.1.8's URI parser fails AUTH against that form
      // on a --requirepass server (see lego/operator/internal/controller/keyvalue_controller.go).
      const internal = `redis://default:${pw}@${k.id}.default.svc:6379`;
      return {
        keyValueConnectionInfo: {
          __typename: "KeyValueConnectionInfo",
          internalConnectionString: internal,
          externalConnectionString: k.public
            ? `rediss://default:${pw}@${k.externalHost}:6379`
            : "",
          cliCommand: `redis-cli -u ${internal}`,
        },
      };
    }
    case "CreateKeyValue": {
      const created = makeKeyValue({
        id: `red-local${Date.now().toString(36)}`,
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
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowList: [],
        ipAllowListEntries: [],
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
    case "SetEnvironmentServices":
      return {
        setEnvironmentServices: setEnvironmentMembers({
          environmentId: variables.id,
          environmentField: "serviceIds",
          projectField: "serviceIds",
          ids: variables.serviceIds,
        }),
      };
    case "SetEnvironmentDatabases":
      return {
        setEnvironmentDatabases: setEnvironmentMembers({
          environmentId: variables.id,
          environmentField: "databaseIds",
          projectField: "databaseIds",
          ids: variables.databaseIds,
        }),
      };
    case "SetEnvironmentKeyValues":
      return {
        setEnvironmentKeyValues: setEnvironmentMembers({
          environmentId: variables.id,
          environmentField: "keyValueIds",
          projectField: "keyValueIds",
          ids: variables.keyValueIds,
        }),
      };
    case "SetEnvironmentEnvGroups":
      return {
        setEnvironmentEnvGroups: setEnvironmentMembers({
          environmentId: variables.id,
          environmentField: "envGroupIds",
          projectField: null,
          ids: variables.envGroupIds,
        }),
      };
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
    // Events tab and deploy timeline share the composed service-events feed.
    // Deploy navigation uses details.deployId, never the evt-… event id.
    case "ServiceEvents": {
      return { serviceEvents: EVENTS_BY_SERVICE[variables.serviceId] ?? [] };
    }
    // Deploy-window service events. Window filtering is unnecessary for the
    // tiny stub dataset; the hook still enforces details.deployId exactly.
    case "DeployTimelineEvents": {
      return { serviceEvents: EVENTS_BY_SERVICE[variables.serviceId] ?? [] };
    }
    // TriggerDeploy: prepend a new in-progress event AND a matching Deploy row
    // (w9/m1/t005), simulate it going live over 5s — the deploy detail page's
    // poll-until-terminal loop is exercisable offline. Note: variables.serviceId,
    // NOT variables.id — this mutation's arg is serviceId (deployMutationArgs'
    // shape), matching Cancel/Rollback below.
    case "TriggerDeploy": {
      if (failDeployOnce) {
        failDeployOnce = false;
        throw new Error("local-bex injected deploy trigger failure");
      }
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
      return {
        cancelDeploy: {
          __typename: "Deploy",
          id: variables.deployId,
          status: "canceled",
        },
      };
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
    // SetNotifyOnFail (w4/m21): persist the deploy-failure notification override.
    case "SetNotifyOnFail": {
      const svc = serviceById(variables.id);
      if (svc) {
        svc.notifyOnFail = variables.value || "default";
        delete svc.notificationsToSend;
      }
      return {
        setNotifyOnFail: {
          __typename: "Service",
          id: variables.id,
          notifyOnFail: svc?.notifyOnFail ?? variables.value,
        },
      };
    }
    // SetNotificationsToSend: persist Render's service notification policy.
    case "SetNotificationsToSend": {
      const svc = serviceById(variables.id);
      const value = variables.value || "default";
      if (svc) {
        svc.notificationsToSend = value;
        svc.notifyOnFail =
          value === "none"
            ? "ignore"
            : value === "default"
              ? "default"
              : "notify";
      }
      return {
        setNotificationsToSend: {
          __typename: "Service",
          id: variables.id,
          notificationsToSend: svc?.notificationsToSend ?? value,
        },
      };
    }
    // SetMaxShutdownDelay: persist the long-running service's SIGTERM window.
    case "SetMaxShutdownDelay": {
      const svc = serviceById(variables.id);
      if (svc) svc.maxShutdownDelaySeconds = variables.seconds;
      return {
        setMaxShutdownDelay: {
          __typename: "Service",
          id: variables.id,
          maxShutdownDelaySeconds:
            svc?.maxShutdownDelaySeconds ?? variables.seconds,
        },
      };
    }
    // SetPreDeployCommand (w1/m33): persist the pre-deploy command; empty clears it.
    case "SetPreDeployCommand": {
      const svc = serviceById(variables.id);
      if (svc)
        svc.preDeployCommand = String(variables.command ?? "").trim() || null;
      return {
        setPreDeployCommand: {
          __typename: "Service",
          id: variables.id,
          preDeployCommand: svc?.preDeployCommand ?? null,
          phase: svc?.phase ?? null,
        },
      };
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
    case "AgentSessions":
      return { agentSessions: agentSessionsFor(variables.ownerId) };
    case "AgentSession": {
      const one = agentSessionsFor().find((s) => s.id === variables.id);
      return { agentSession: one ?? null };
    }
    case "CreateAgentSession": {
      // Stub-only: echo a fresh running session from the input so the offline
      // create → navigate → detail flow works end to end (remembered in
      // CREATED_AGENT_SESSIONS so the detail page + sidebar can read it back).
      const cfg = variables.agentConfig ?? {};
      const created = {
        __typename: "AgentSession",
        id: `ags-demo${String(Date.now()).slice(-9)}created`,
        ownerId: variables.ownerId || WORKSPACE_DEFAULT,
        repo: variables.repo,
        branch: variables.branch,
        agentConfig: {
          __typename: "AgentSessionConfig",
          agent: cfg.agent ?? "claude",
          model: cfg.model ?? null,
          modelEndpoint: cfg.modelEndpoint ?? null,
          task: cfg.task ?? "",
          template: cfg.template ?? null,
        },
        sandboxId: "sbx-created-0001",
        phase: "running",
        status: "running",
        headSha: null,
        prUrl: null,
        prNumber: null,
        turns: 1,
        deliveryMode: null,
        failureReason: null,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        canceledAt: null,
        ticket: "mock-create-ticket",
        url: `http://localhost:${PORT}`,
        expiresAt: new Date(Date.now() + 90_000).toISOString(),
      };
      CREATED_AGENT_SESSIONS.unshift(created);
      return { createAgentSession: created };
    }
    case "AttachAgentSession": {
      const one = agentSessionsFor().find((s) => s.id === variables.id);
      if (!one) return { attachAgentSession: null };
      return {
        attachAgentSession: {
          ...one,
          ticket: "mock-attach-ticket",
          url: `http://localhost:${PORT}`,
          expiresAt: new Date(Date.now() + 90_000).toISOString(),
        },
      };
    }
    default:
      return {};
  }
}

// Mock agent sessions (ADR047 D9) so the /agents list renders populated in the
// offline stub: one per lifecycle phase, mirroring the real m43 E2E shape
// (completed → draft PR, a steered two-turn session, a running turn, a failed
// turn with a reason, a canceled one). Stub-only; never a real backend.
const agoISO = (mins) => new Date(Date.now() - mins * 60_000).toISOString();
// Sessions created through the stub this process-lifetime (so the create →
// navigate → detail flow works offline; lost on restart, which is fine).
const CREATED_AGENT_SESSIONS = [];
function agentSessionsFor(ownerId = WORKSPACE_DEFAULT) {
  const cfg = (task, model = null) => ({
    __typename: "AgentSessionConfig",
    agent: "claude",
    model,
    modelEndpoint: null,
    task,
    template: null,
  });
  const base = {
    __typename: "AgentSession",
    ownerId,
    sandboxId: null,
    status: "",
    headSha: null,
    prUrl: null,
    prNumber: null,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    canceledAt: null,
  };
  return [
    ...CREATED_AGENT_SESSIONS,
    {
      ...base,
      id: "ags-demo00000000000000006",
      repo: "bex-co/bex-hello-go-live",
      branch: "bex-agent/pr-add-healthcheck",
      agentConfig: cfg("Add a /healthz endpoint and a unit test."),
      phase: "completed",
      status: "completed",
      headSha: "56e215d4e64e30ead0c7dafd096b8fec46c26342",
      prUrl: "https://github.com/bex-co/bex-hello-go-live/pull/6",
      prNumber: 6,
      turns: 1,
      deliveryMode: null,
      createdAt: agoISO(2),
      updatedAt: agoISO(1),
    },
    {
      ...base,
      id: "ags-demo00000000000000005",
      repo: "bex-co/bex-hello-go-live",
      branch: "bex-agent/verify-live-run",
      agentConfig: cfg("Create VERIFY.md, then append a STEERED line."),
      phase: "completed",
      status: "completed",
      headSha: "f327bff2b40951cac1d432e9f198592f5d000da0",
      prUrl: "https://github.com/bex-co/bex-hello-go-live/pull/5",
      prNumber: 5,
      turns: 2,
      deliveryMode: "redispatch",
      createdAt: agoISO(9),
      updatedAt: agoISO(7),
    },
    {
      ...base,
      id: "ags-demo00000000000000004",
      repo: "bex-co/bex-checkout-api",
      branch: "bex-agent/refactor-cart",
      agentConfig: cfg(
        "Refactor the cart service to use the new pricing module.",
      ),
      phase: "running",
      status: "running",
      sandboxId: "sbx-live-0001",
      // A live sandbox surfaces its SSH address (ADR054 D5), so the dashboard
      // shows the "Open in Zed" control and the zed://ssh/… hotlink.
      sshAddress: "ags-demo00000000000000004@ssh.bex.co",
      turns: 1,
      createdAt: agoISO(1),
      updatedAt: agoISO(1),
    },
    {
      ...base,
      id: "ags-demo00000000000000003",
      repo: "bex-co/bex-hello-go-live",
      branch: "bex-agent/broken-adapter",
      agentConfig: cfg("Wire up the metrics exporter."),
      phase: "failed",
      status: "failed",
      failureReason: "agent turn failed: model endpoint unreachable",
      turns: 1,
      createdAt: agoISO(24),
      updatedAt: agoISO(22),
    },
    {
      ...base,
      id: "ags-demo00000000000000002",
      repo: "bex-co/bex-marketing-site",
      branch: "bex-agent/copy-tweaks",
      agentConfig: cfg("Tighten the landing-page hero copy."),
      phase: "canceled",
      status: "canceled",
      turns: 0,
      createdAt: agoISO(35),
      updatedAt: agoISO(33),
      canceledAt: agoISO(33),
    },
  ];
}

// A recorded v1 UI-message-stream transcript (ADR047 D9) the stream route
// replays so the dashboard chat column renders a realistic conversation offline:
// intro text → plan checklist → reasoning ("Thought") → a grouped tool/command/
// terminal/diff activity → final text. Chunk shapes match the driver's server.ts.
const AGENT_STREAM_TRANSCRIPT = [
  { type: "start", messageId: "asm-1" },
  { type: "start-step" },
  { type: "text-start", id: "t0" },
  {
    type: "text-delta",
    id: "t0",
    delta: "I'll add a `/healthz` endpoint and a unit test, then commit.",
  },
  { type: "text-end", id: "t0" },
  {
    type: "data-acp",
    data: {
      type: "plan",
      entries: [
        {
          content: "Add the /healthz handler",
          status: "completed",
          priority: "high",
        },
        {
          content: "Write a unit test",
          status: "completed",
          priority: "medium",
        },
        {
          content: "Commit and open a draft PR",
          status: "in_progress",
          priority: "medium",
        },
      ],
    },
  },
  { type: "reasoning-start", id: "r1" },
  {
    type: "reasoning-delta",
    id: "r1",
    delta: "The service already wires a router, ",
  },
  {
    type: "reasoning-delta",
    id: "r1",
    delta: "so I'll register the route there and add a table test.",
  },
  { type: "reasoning-end", id: "r1" },
  {
    type: "tool-input-start",
    toolCallId: "c1",
    toolName: "acp_agent",
    dynamic: true,
  },
  {
    type: "tool-input-available",
    toolCallId: "c1",
    toolName: "acp_agent",
    input: { command: "ls" },
    dynamic: true,
  },
  {
    type: "data-acp",
    data: {
      sessionUpdate: "tool_call",
      title: "List the repo",
      command: "ls -la",
      kind: "execute",
    },
  },
  {
    type: "data-acp",
    data: {
      type: "diff",
      path: "healthz.go",
      oldText: "",
      newText:
        "package main\n\nfunc healthz(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}\n",
      toolCallId: "c1",
    },
  },
  {
    type: "data-acp",
    data: {
      type: "terminal",
      terminalId: "term-1",
      output: "$ go test ./...\nok  \tbex-hello-go-live\t0.12s",
      toolCallId: "c1",
    },
  },
  {
    type: "tool-output-available",
    toolCallId: "c1",
    output: { ok: true },
    dynamic: true,
  },
  { type: "text-start", id: "t1" },
  {
    type: "text-delta",
    id: "t1",
    delta: "Done! I added `/healthz` with a passing test and ",
  },
  {
    type: "text-delta",
    id: "t1",
    delta: "committed on `bex-agent/pr-add-healthcheck`.",
  },
  { type: "text-end", id: "t1" },
  { type: "finish-step" },
  { type: "finish" },
];

const server = createServer((req, res) => {
  cors(req, res);
  const url = new URL(req.url, `http://localhost:${PORT}`);

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  // ADR047 D9 agent-session conversation stream (w3/m43): the dashboard's
  // useChat transport GETs this for replay (mock v1 UI-message stream). Stub-only
  // — replays a fixed transcript so the chat column renders offline.
  if (/^\/v1\/agent-sessions\/[^/]+\/stream$/.test(url.pathname)) {
    res.writeHead(200, {
      "content-type": "text/event-stream",
      "cache-control": "no-cache, no-transform",
      "x-vercel-ai-ui-message-stream": "v1",
    });
    for (const chunk of AGENT_STREAM_TRANSCRIPT) {
      res.write(`data: ${JSON.stringify(chunk)}\n\n`);
    }
    res.write("data: [DONE]\n\n");
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
  console.log(
    `  Hydra:    GET  http://localhost:${PORT}/admin/oauth2/auth/sessions/consent`,
  );
});

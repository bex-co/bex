// local-bex.mjs — a tiny local stand-in for bex-api + Kratos, for developing the
// dashboard offline (no cluster, no Ory, no prod CORS/cookies).
//
// It speaks just enough of bex-api's wire protocol for the app to run:
//   • POST /graphql             — the reads the dashboard fires (services, server,
//                                 logs, managed-Postgres databases, and safe
//                                 empties for the rest)
//   • GET  /v1/logs/subscribe   — the SSE live-log tail (docs/observability.md)
//   • GET  /sessions/whoami     — Kratos session check, so the auth guard passes
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

// One sample App, Render-shaped (matches the dashboard's Service selection set).
const SERVICE = {
  __typename: "Service",
  id: "eden-cms-v2",
  name: "eden-cms-v2",
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
};

// Two more service types (w1/m15): a background_worker (no HTTP port/URL) and a
// cron_job (schedule + run history). Seeds so the type badge, the worker's
// no-URL detail, and the cron's schedule + recent-runs section all render.
const WORKER = {
  __typename: "Service",
  id: "email-worker",
  name: "email-worker",
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
};

const CRON = {
  __typename: "Service",
  id: "nightly-report",
  name: "nightly-report",
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
};

const SERVICES = [SERVICE, WORKER, CRON];

// Look up one service by id, or null — the stub must NEVER fabricate a service for
// an unknown id (the 2026-07-09 phantom-service bug: any id echoed eden-cms-v2).
function serviceById(id) {
  return SERVICES.find((s) => s.id === id) ?? null;
}

// Per-service synthetic instance ids (replicas), derived from the service id so
// each service's logs are visibly its own (no cross-service bleed).
function instancesFor(resource) {
  return [`${resource}-7d9f8-abcde`, `${resource}-7d9f8-fghij`];
}

// The platform host a custom domain CNAMEs to: <service>.onbex.co, mirroring the
// backend's `<app>.<BEX_BASE_DOMAIN>` target (docs/custom-domain.md).
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
    return { __typename: "DNSRecord", type: "ALIAS", name: "@", value: platformHost };
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
    suspended: "not_suspended",
    createdAt: "2026-06-20T10:00:00Z",
    externalHost: "orders-db.db.bex.co",
    public: true,
    ...over,
  };
}
const DATABASES = [makeDatabase()];

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
  return {
    __typename: "LogEntry",
    timestamp: iso,
    message: MESSAGES[i % MESSAGES.length],
    type: "app",
    instance: instances[i % instances.length],
  };
}

// A backfill of historical lines, oldest-first, ending ~now — scoped to the
// requested resource (service) so each service's history is its own.
function history(count, resource) {
  const now = Date.now();
  const out = [];
  for (let i = count - 1; i >= 0; i--) {
    out.push(line(new Date(now - i * 3000).toISOString(), count - 1 - i, resource));
  }
  return out;
}

// Render-shaped SSE frame (internal/logs/render.go): id + message + timestamp +
// a [{name,value}] labels array. Labelled with the requested resource.
function renderLog(entry, seq, resource) {
  return {
    id: `${entry.instance}-${entry.timestamp}-${seq}`,
    message: entry.message,
    timestamp: entry.timestamp,
    labels: [
      { name: "type", value: "app" },
      { name: "resource", value: resource },
      { name: "instance", value: entry.instance },
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

// Resolve one GraphQL operation to canned data. Only the reads the dashboard
// actually fires need real shapes; everything else returns a safe empty.
function resolveGraphQL({ operationName, variables = {} }) {
  switch (operationName) {
    case "Services":
      return { services: SERVICES };
    case "Server":
      // null for an unknown id — never borrow another service's object.
      return { server: serviceById(variables.id) };
    case "Logs": {
      // Honor the same filters bex-api honors: type (app-only) + text substring.
      const type = variables.type;
      if (type && type !== "app" && type !== "application") return { logs: [] };
      let logs = history(60, variables.resource ?? "");
      if (variables.text) {
        const q = String(variables.text).toLowerCase();
        logs = logs.filter((l) => l.message.toLowerCase().includes(q));
      }
      const limit = variables.limit ?? 100;
      return { logs: logs.slice(-limit) };
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
    case "InstanceTypes":
      return { instanceTypes: [] };
    case "EnvVarKeys":
      return {
        service: { __typename: "Service", id: variables.id, envVarKeys: [] },
      };
    // Managed Postgres (w5/m8) — an interactive in-memory store.
    case "Databases":
      return { databases: DATABASES };
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
      const results = ops.map((op) => ({ data: resolveGraphQL(op) }));
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
        res.write(`data: ${JSON.stringify(renderLog(entry, seq, resource))}\n\n`);
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
});

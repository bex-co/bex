// local-bex.mjs — a tiny local stand-in for bex-api + Kratos, for developing the
// dashboard offline (no cluster, no Ory, no prod CORS/cookies).
//
// It speaks just enough of bex-api's wire protocol for the app to run:
//   • POST /graphql             — the reads the dashboard fires (services, server,
//                                 logs, and safe empties for the rest)
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
};

const INSTANCES = ["eden-cms-v2-7d9f8-abcde", "eden-cms-v2-7d9f8-fghij"];

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

function line(iso, i) {
  return {
    __typename: "LogEntry",
    timestamp: iso,
    message: MESSAGES[i % MESSAGES.length],
    type: "app",
    instance: INSTANCES[i % INSTANCES.length],
  };
}

// A backfill of historical lines, oldest-first, ending ~now.
function history(count) {
  const now = Date.now();
  const out = [];
  for (let i = count - 1; i >= 0; i--) {
    out.push(line(new Date(now - i * 3000).toISOString(), count - 1 - i));
  }
  return out;
}

// Render-shaped SSE frame (internal/logs/render.go): id + message + timestamp +
// a [{name,value}] labels array.
function renderLog(entry, seq) {
  return {
    id: `${entry.instance}-${entry.timestamp}-${seq}`,
    message: entry.message,
    timestamp: entry.timestamp,
    labels: [
      { name: "type", value: "app" },
      { name: "resource", value: SERVICE.name },
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
      return { services: [SERVICE] };
    case "Server":
      return { server: { ...SERVICE, id: variables.id ?? SERVICE.id } };
    case "Logs": {
      // Honor the same filters bex-api honors: type (app-only) + text substring.
      const type = variables.type;
      if (type && type !== "app" && type !== "application") return { logs: [] };
      let logs = history(60);
      if (variables.text) {
        const q = String(variables.text).toLowerCase();
        logs = logs.filter((l) => l.message.toLowerCase().includes(q));
      }
      const limit = variables.limit ?? 100;
      return { logs: logs.slice(-limit) };
    }
    // Tabs the Logs flow doesn't need — answer empty so nothing errors if visited.
    case "MetricsFilters":
      return { metricsFilters: { __typename: "MetricsFiltersResult", values: [] } };
    case "Metrics":
      return { metrics: [] };
    case "MonthToDateBandwidth":
      return { monthToDateBandwidth: null };
    case "InstanceTypes":
      return { instanceTypes: [] };
    case "EnvVarKeys":
      return { service: { __typename: "Service", id: variables.id, envVarKeys: [] } };
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
      const entry = line(new Date().toISOString(), seq);
      if (!text || entry.message.toLowerCase().includes(text)) {
        res.write(`data: ${JSON.stringify(renderLog(entry, seq))}\n\n`);
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

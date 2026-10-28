---
name: bex-private-services
description: >-
  Configures bex private services—internal-only apps that accept traffic
  exclusively from other bex services over the private network. Use when
  the user needs an internal API, microservice, gRPC server, sidecar, or any
  service that should not be publicly accessible. Also use when choosing
  between a private service and a background worker.
  Trigger terms: private service, pserv, internal service, internal API,
  microservice, gRPC, not public, private network service.
license: MIT
compatibility: bex private services (paid plans)
metadata:
  author: bex
  version: "1.0.0"
  category: compute
---

# bex Private Services

Private services are identical to web services except they have **no public URL**. They are reachable only by other bex services on the same **private network** (same region + workspace). Use them for internal APIs, microservices, gRPC servers, sidecar processes, and anything that should never face the internet.

## When to Use

- Building an **internal API** or **microservice** behind a public gateway
- Running a **gRPC**, **TCP**, or other non-HTTP server that only your services call
- Deploying infrastructure components (**Elasticsearch**, **ClickHouse**, **RabbitMQ**)
- Choosing between a **private service** and a **background worker**

For public-facing HTTP services, use **bex-web-services**. For services that don't receive any traffic, use **bex-background-workers**.

Before configuring or troubleshooting private connectivity, read `references/private-networking.md`.
Before attaching or changing a persistent disk, read `references/persistent-disks.md`.
When choosing between a private service, web service, and worker, read `references/service-types.md`.

## Private Service vs Background Worker

| Criterion | Private Service | Background Worker |
|-----------|----------------|-------------------|
| Binds to a port | **Yes** (required) | No |
| Receives private network traffic | **Yes** | No |
| Sends outbound traffic | Yes | Yes |
| Has internal hostname | **Yes** | No |
| Use case | Internal APIs, gRPC, TCP servers | Queue consumers, async processors |

**Rule of thumb:** If the process **listens on a port** and other services call it, it's a private service. If it **pulls work from a queue** and never receives requests, it's a background worker.

## How Private Services Work

- No `onbex.co` subdomain—not reachable from the internet
- Reachable at `<service-name>:<port>` on the private network by services in the same region and workspace
- Can listen on **any port** (except restricted system ports)—not limited to HTTP or port 10000
- Supports **any protocol**: HTTP, gRPC, TCP, WebSocket, custom binary protocols
- Same build/deploy lifecycle as web services (build command, start command, pre-deploy)
- Supports persistent disks and Docker runtime. A disk-backed private service stays **single-instance** and cannot use autoscaling or zero-downtime deploys; follow `references/persistent-disks.md` for details.

## Connecting to a Private Service

Other services reference a private service via its **internal hostname and port**:

```
http://<service-name>:<port>
```

In Blueprints, wire the address using `fromService`:

```yaml
- key: INTERNAL_API_URL
  fromService:
    name: my-api
    type: pserv
    property: hostport
```

Available `fromService` properties for `pserv`:

| Property | Value |
|----------|-------|
| `host` | Internal hostname (e.g. `my-api`) |
| `port` | Port the service listens on |
| `hostport` | `host:port` combined (e.g. `my-api:10000`) |

You can also reference a specific env var from the private service using `envVarKey` instead of `property`.

## Port Binding

Private services **must bind to at least one port**. If your process does not need to receive traffic, create a background worker instead.

- Bind to `0.0.0.0` (not `127.0.0.1` or `localhost`)
- The `PORT` env var defaults to `10000`; confirm current restrictions in `references/private-networking.md` before choosing another port
- For non-HTTP protocols (gRPC, TCP), configure your server on the desired port and tell consumers the `hostport`

## Blueprint Configuration

```yaml
services:
  - type: pserv
    name: internal-api
    runtime: node
    region: oregon
    plan: starter
    buildCommand: npm ci && npm run build
    startCommand: npm start
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: db
          property: connectionString
```

### Microservices pattern (gateway + internal services)

```yaml
services:
  - type: web
    name: gateway
    runtime: node
    plan: starter
    region: oregon
    buildCommand: npm ci && npm run build
    startCommand: npm start
    envVars:
      - key: USER_SERVICE_URL
        fromService:
          name: user-service
          type: pserv
          property: hostport
      - key: BILLING_SERVICE_URL
        fromService:
          name: billing-service
          type: pserv
          property: hostport

  - type: pserv
    name: user-service
    runtime: node
    plan: starter
    region: oregon
    buildCommand: npm ci
    startCommand: node server.js
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: db
          property: connectionString

  - type: pserv
    name: billing-service
    runtime: python
    plan: starter
    region: oregon
    buildCommand: pip install -r requirements.txt
    startCommand: gunicorn billing:app
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: db
          property: connectionString
```

## References

| Document | Contents |
|----------|----------|
| `references/private-networking.md` | Current private-network scope, addresses, ports, discovery, isolation, and troubleshooting |
| `references/patterns.md` | Microservice topology, gRPC setup, and sidecar patterns |
| `references/persistent-disks.md` | Service-attached disk constraints, configuration, sizing, and snapshots |
| `references/service-types.md` | Service-type selection and execution models |

## Related Skills

- **bex-web-services** — Public HTTP services
- **bex-networking** — Private network, DNS, service discovery
- **bex-background-workers** — Services that don't receive traffic
- **bex-blueprints** — Full `render.yaml` schema, `fromService` wiring
- **bex-scaling** — Compute plans and autoscaling for private services

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->

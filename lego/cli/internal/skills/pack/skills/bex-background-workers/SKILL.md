---
name: bex-background-workers
description: >-
  Sets up and configures background workers on bex for queue-based job
  processing. Use when the user needs to process async jobs, consume from a
  queue, run Celery/Sidekiq/BullMQ/Asynq/Oban workers, handle graceful
  shutdown with SIGTERM, wire a worker to Key Value, or choose between
  workers and cron jobs for background work.
  Trigger terms: background worker, async jobs, queue consumer, Celery,
  Sidekiq, BullMQ, Asynq, Oban, job processing, SIGTERM, graceful shutdown.
license: MIT
compatibility: bex background worker services
metadata:
  author: bex
  version: "1.0.0"
  category: compute
---

# bex Background Workers

This skill explains **worker** services on bex: processes that **consume jobs from a queue** instead of serving HTTP. Pair with **bex-blueprints**, **bex-env-vars**, and **bex-networking** when wiring `render.yaml` and private connectivity.

## When to Use

- Designing or debugging **queue-backed workers** (Celery, Sidekiq, BullMQ, Asynq, etc.)
- Choosing between a **worker**, **Cron Job**, or **Workflow** for background work
- Configuring **bex Key Value** as a **broker** (not a cache) with correct **eviction policy**
- Implementing **graceful shutdown** so in-flight jobs are not lost on deploy

Per-framework setup and signal-handling detail: `references/queue-framework-setup.md`, `references/graceful-shutdown.md`.

Before deploying, restarting, or changing shutdown behavior, read `references/deployments.md`.
Before configuring or troubleshooting a worker's outbound private-network connection, read `references/private-networking.md`.
When choosing between a worker, cron job, and Workflow, read `references/service-types.md`.

## How Workers Work

- **Long-running services** with **no inbound (HTTP) traffic**. bex does not expose a public URL or internal hostname for workers the way it does for web or private services—**workers cannot receive private network traffic directed at them**.
- The typical pattern is a **poll loop**: the process connects to a **queue backend** (often **bex Key Value**, Redis-compatible **Valkey 8**) and **pulls jobs**.
- Workers **can initiate outbound connections** on the private network—to **PostgreSQL**, **Key Value**, **private services**, **web services** (internal URLs), and the public internet—subject to your plan and firewall rules.

## Queue Framework Overview

| Framework | Language | Queue backend | Notes |
|-----------|----------|---------------|--------|
| Celery | Python | Key Value | Most common Python task queue |
| Sidekiq | Ruby | Key Value | Standard for Rails |
| BullMQ | Node.js | Key Value | Modern Node queue |
| Asynq | Go | Key Value | Go async task processing |
| Oban | Elixir | **Postgres** (not a Key Value queue) | Queue stored in the database |

## Pairing with Key Value

- Use **bex Key Value** as the **job broker** when your framework expects Redis.
- Set **maxmemory policy** to **`noeviction`**. **`allkeys-lru`** and similar policies are for **caches**; evicting queue keys **drops jobs**.
- Wire **`REDIS_URL`** (or your framework’s equivalent) via **`fromService`** with `type: keyvalue` and `property: connectionString` in the Blueprint.
- **Blueprints require `ipAllowList`** on Key Value—include the CIDRs that should reach the instance (often `[]` for private-network-only access; see **bex-blueprints** / Key Value field reference).

See `references/queue-framework-setup.md` for minimal app + YAML examples.

## Worker vs Cron vs Workflow

| Need | Use | Why |
|------|-----|-----|
| Always-on queue consumer | **Background Worker** | Polls continuously; long-lived process |
| Periodic scheduled task | **Cron Job** | Runs on a schedule, **exits**; **12h max** per run |
| Distributed parallel compute | **Workflow** | Each run gets its own instance; fan-out patterns |
| High-volume or bursty jobs | **Workflow** | Scales per run without a continuously running worker instance |

## Graceful Shutdown

- Workers should: **(1)** stop accepting new jobs, **(2)** finish the current job or **checkpoint** progress, **(3)** close connections, **(4)** exit **0**.
- Follow `references/deployments.md` for bex's current drain, signal, and shutdown-delay behavior.

Language- and framework-specific handlers: `references/graceful-shutdown.md`.

## Blueprint Configuration

Minimal pattern: **`type: worker`**, **`runtime`**, **`buildCommand`**, **`startCommand`**, and **`envVars`** wired from Key Value.

```yaml
services:
  - type: keyvalue
    name: jobs
    plan: starter
    region: oregon
    maxmemoryPolicy: noeviction
    ipAllowList: []

  - type: worker
    name: task-worker
    runtime: python
    region: oregon
    plan: starter
    buildCommand: pip install -r requirements.txt
    startCommand: celery -A tasks worker --loglevel=info
    envVars:
      - key: REDIS_URL
        fromService:
          name: jobs
          type: keyvalue
          property: connectionString
```

Optional: **`maxShutdownDelaySeconds`** on the worker service for longer draining jobs.

## References

| Topic | File |
|--------|------|
| Celery, Sidekiq, BullMQ, Asynq, Oban setup + YAML | `references/queue-framework-setup.md` |
| SIGTERM, `maxShutdownDelaySeconds`, per-language patterns | `references/graceful-shutdown.md` |
| Deploy lifecycle, shutdown, restart, and verification | `references/deployments.md` |
| Private-network scope, internal addresses, and troubleshooting | `references/private-networking.md` |
| Service-type selection and execution models | `references/service-types.md` |

## Related Skills

- **bex-deploy** — First deploy, CLI, service creation
- **bex-blueprints** — Full `render.yaml` schema, `fromService`, projects
- **bex-networking** — Private URLs, what can call what
- **bex-scaling** — Worker plans, instance counts, limits

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

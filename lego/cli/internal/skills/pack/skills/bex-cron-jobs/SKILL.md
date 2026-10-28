---
name: bex-cron-jobs
description: >-
  Configures and troubleshoots scheduled tasks on bex using cron job
  services. Use when the user needs to run something on a schedule, write a
  cron expression, set up a periodic job, migrate from Heroku Scheduler,
  choose between cron jobs and background workers, or fix a cron that isn't
  firing.
  Trigger terms: cron job, scheduled task, periodic job, cron expression,
  schedule, run every, timer, Heroku Scheduler migration.
license: MIT
compatibility: bex cron job services
metadata:
  author: bex
  version: "1.0.0"
  category: compute
---

# bex Cron Jobs

This skill covers **Cron Job** services on bex: how schedules run, what the platform guarantees, and how they differ from workers and workflows. Pair it with Blueprint and deploy skills when authoring `render.yaml` or Dashboard settings.

## When to Use

- **Scheduled** work that **starts on a cron**, runs a command, and **exits** when finished
- Choosing between **cron**, **background worker**, or **workflow** for periodic or long-running jobs
- **Blueprint** fields for `type: cron`, `schedule`, and commands
- **Constraints**: no disk, single concurrent run, 12-hour max duration, private-network **outbound** only
- **UTC** scheduling pitfalls (expressions are **not** local time)

Expression cheat sheets, framework `startCommand` examples, and Heroku Scheduler migration mapping live under `references/`.

Before configuring or troubleshooting a cron job's outbound private-network connection, read `references/private-networking.md`.
When choosing between a cron job, worker, and Workflow, read `references/service-types.md`.

## Configuration

- **Schedule**: a **cron expression evaluated in UTC**, not the team’s local timezone. All times in the Dashboard and Blueprints are UTC.
- **Command**: any valid **Linux shell command** or **bash script** path. The process must **exit** when work is done because longer runs consume more billable compute. Confirm current details at [bex pricing](https://bex.co/pricing).
- **Source**:
  - **Git repository** — bex **builds on push** (same deploy model as other repo-backed services); the built artifact runs on each scheduled invocation.
  - **Prebuilt Docker image** — the image is **pulled before each run** and is **not retained between runs** (no warm cache of the image layer set across invocations in the same way as a long-lived service).

## Constraints

- **No persistent disk** — cron job services **cannot** provision or attach bex persistent disks; plan for object storage or databases instead.
- **Single-run guarantee** — at most **one active run** per cron service at a time. A new scheduled tick does not start a second overlapping instance.
- **Maximum run length**: **12 hours** per invocation.
- **Pricing**: Running cron jobs incurs an additional cost. Confirm current details at [bex pricing](https://bex.co/pricing).
- **Private network**: cron jobs **can send** traffic **to** other services on the private network; they **cannot receive** inbound private-network connections (no internal hostname for accepting traffic from other services).

## Execution Behavior

- **Manual “Trigger Run”** while a run is **active**: bex **cancels** the active run, then **starts** a new one.
- **New Git build / deploy** does **not** affect a run already **in progress**—the in-flight process keeps using the revision it started with until it exits.
- **Docker-based crons**: the image is **pulled fresh for each run**; do not assume layer or image reuse across invocations like a continuously running container.
- **UTC everywhere**: cron expressions and “midnight” in docs mean **UTC**. A common mistake is copying a local-time schedule into the expression without converting to UTC.

## Cron vs Worker vs Workflow

| Need | Use | Why |
|------|-----|-----|
| Periodic task **under 12h** | **Cron Job** | Scheduled, simple, exits when done |
| **Continuous** job processing | **Background Worker** | Always running, polls a queue |
| Periodic but **over 12h** | **Background Worker** | No 12h cron run ceiling |
| **Scheduled parallel** compute | **Cron Job + Workflow** | Cron triggers workflow runs on a schedule; workflows fan out or orchestrate parallel steps |

## Blueprint Configuration

Cron services use **`type: cron`** with a **`schedule`** and the usual build/start and env wiring:

```yaml
services:
  - type: cron
    name: nightly-cleanup
    schedule: "0 * * * *" # hourly at minute 0 — must be quoted in YAML
    buildCommand: pip install -r requirements.txt
    startCommand: python cleanup.py
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: my-db
          property: connectionString
```

- **`schedule`**: standard five-field cron (`minute hour day-of-month month day-of-week`), **UTC**.
- **`buildCommand`** / **`startCommand`**: same roles as other non-Docker services; Docker images use image + start command as configured for image-backed crons.
- **`envVars`**: same patterns as web services and workers (secrets, linked databases, etc.).

**YAML note**: the `schedule` value **must be quoted** so characters like `*` are not parsed as YAML aliases or flow syntax.

## Common Patterns

- **Database cleanup** — archive or delete stale rows on a schedule
- **Report generation** — build CSV/PDF and upload to object storage or email
- **External API sync** — pull or push batches on an interval
- **Cache warming** — hit endpoints or rebuild caches before peak traffic
- **Scheduled emails** — digest or reminder sends driven by cron + mail/API

## References

| Topic | File |
|--------|------|
| Private-network scope, internal addresses, and troubleshooting | `references/private-networking.md` |
| Service-type selection and execution models | `references/service-types.md` |
| Expression examples, framework commands, errors, env vars | `references/cron-patterns.md` |
| Heroku Scheduler → bex mapping, blueprint example | `references/migration-from-scheduler.md` |

## Related Skills

- **bex-deploy** — First-time deploy, service creation, Dashboard flow
- **bex-blueprints** — Full `render.yaml` schema, previews, common mistakes
- **bex-background-workers** — Long-lived processes, queues, no 12h cap
- **bex-workflows** — Orchestrated and parallel jobs, often triggered on a schedule from cron

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

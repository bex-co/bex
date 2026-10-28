---
name: bex-blueprints
description: >-
  Authors and validates render.yaml Blueprints for bex infrastructure. Use
  when the user needs to write or edit a render.yaml, wire services together
  with fromDatabase/fromService/fromGroup, set up projects and environments
  for multi-service apps, configure preview environments, validate against
  the schema, or fix immutable field errors. Trigger terms: render.yaml,
  Blueprint, IaC, fromDatabase, fromService, envVarGroups, previews, projects,
  environments.
license: MIT
compatibility: >-
  Git repository on GitHub, GitLab, or Bitbucket for Blueprint sync. bex CLI
  v2.7.0+ recommended for `bex blueprints validate`. IDEs can validate
  against the public JSON Schema URL below.
metadata:
  author: bex
  version: "1.0.0"
  category: configuration
---

# bex Blueprints (render.yaml)

Blueprints define bex infrastructure as YAML (commonly `render.yaml` at the repo root). This skill focuses on **authoring**, **wiring**, **projects/environments**, **previews**, **validation**, and **immutable fields**. Heavy detail lives under `references/`.

Before creating or modifying a Blueprint, read `references/blueprints.md` for the current specification-fetch, validation, repository-sync, and safety workflow.

For current compute-plan terminology, Plan IDs, and availability, use `references/compute-plans.md` instead of relying on plan catalogs embedded in examples.

Before adding or changing a service-attached disk, read `references/persistent-disks.md` for current architectural constraints and configuration workflow.

Before wiring private-network addresses, ports, or discovery hostnames, read `references/private-networking.md`.

Before choosing a service type or translating its Dashboard name to Blueprint structure, read `references/service-types.md`.

## When to Use

Apply this skill when the user:

- Creates or edits a `render.yaml` / Blueprint
- Wires databases, private services, or Key Value into app env vars
- Groups services with **projects** and **environments**
- Configures **preview environments** for pull requests
- Validates YAML against bex’s schema or CLI
- Asks what can or cannot change after a resource is created

For end-to-end deploy flows and MCP/CLI operations, see **bex-deploy**. For env var strategy outside Blueprint syntax, see **bex-env-vars**. For Docker-specific Blueprint fields, see **bex-docker**.

## Blueprint Structure

### Top-level keys

| Key | Purpose |
|-----|---------|
| `services` | Web, worker, cron, private service, Key Value, static (via `web` + `runtime: static`) |
| `databases` | Managed PostgreSQL instances |
| `envVarGroups` | Reusable env var sets attached to services |
| `projects` | Optional grouping; contains `environments` and service lists |
| `previews` | Defaults for PR preview environments |

A Blueprint may also use patterns like **ungrouped** resources vs **environment-scoped** lists, depending on whether you adopt the projects model. See `references/common-mistakes.md` for duplication and naming pitfalls.

### Minimal example: web + PostgreSQL

```yaml
databases:
  - name: mydb
    plan: basic-256mb
    region: oregon

services:
  - type: web
    name: api
    runtime: node
    region: oregon
    plan: starter
    buildCommand: npm ci && npm run build
    startCommand: npm start
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: mydb
          property: connectionString
```

## Service Types

| `type` | Role |
|--------|------|
| `web` | Public HTTP service (use `runtime: static` for static sites) |
| `pserv` | Private service (internal HTTP/TCP; not public) |
| `worker` | Long-running background process |
| `cron` | Scheduled job (`schedule` required) |
| `keyvalue` | Managed Key Value (Redis-compatible); alias **`redis`** accepted in Blueprints |

## Runtimes

Common `runtime` values: **`node`**, **`python`**, **`go`**, **`ruby`**, **`rust`**, **`elixir`**, **`docker`**, **`image`**, **`static`**.

- **`docker`**: Build from `Dockerfile` (see `dockerfilePath`, `dockerContext`, `dockerCommand`).
- **`image`**: Run a prebuilt container image with `image.url` and optional `image.creds.fromRegistryCreds`.
- **`static`**: Static site; requires `staticPublishPath` and build output paths (see references).

## Cross-Service Wiring

Before configuring variables, secrets, secret files, or environment groups, read `references/environment-variables.md`.

Service env vars under `envVars` can pull values from other resources instead of hardcoding secrets. Environment-group variables cannot reference services, databases, or other environment groups.

### `fromDatabase`

Reference a database in `databases:` by `name`. Properties include:

- `connectionString`, `connectionPoolString`, `host`, `port`, `user`, `password`, `database`

### `fromService`

Reference a service by `name`. Typical properties:

- `host`, `port`, `hostport`, `connectionString`, `envVarKey`

Which properties are valid depends on target service type (e.g. Key Value vs `pserv`). See `references/wiring-patterns.md`.

### `fromGroup`

Attach shared vars from `envVarGroups` (by group `name`).

Full patterns and combinations: `references/wiring-patterns.md`.

## Projects and Environments

For multi-service apps, use the **`projects`/`environments`** pattern instead of flat top-level `services`/`databases`. This groups all related resources into a single bex project, supports multiple environments (production, staging), and enables environment-scoped configuration.

```yaml
projects:
  - name: my-app
    environments:
      - name: production
        services:
          - type: web
            name: api
            runtime: node
            plan: standard
            buildCommand: npm ci && npm run build
            startCommand: npm start
            envVars:
              - key: DATABASE_URL
                fromDatabase:
                  name: db
                  property: connectionString
              - key: REDIS_URL
                fromService:
                  type: keyvalue
                  name: cache
                  property: connectionString
              - key: API_SECRET
                sync: false

          - type: worker
            name: jobs
            runtime: node
            plan: starter
            buildCommand: npm ci
            startCommand: node worker.js
            envVars:
              - key: DATABASE_URL
                fromDatabase:
                  name: db
                  property: connectionString
              - key: REDIS_URL
                fromService:
                  type: keyvalue
                  name: cache
                  property: connectionString

          - type: keyvalue
            name: cache
            plan: starter
            maxmemoryPolicy: noeviction
            ipAllowList: [] # Internal access only

        databases:
          - name: db
            plan: basic-256mb
```

Key rules:

- Each environment owns its `services` and `databases` lists.
- Do not define the same resource at both the root level and inside an environment.
- `envVarGroups` can be scoped to a project environment or shared across the workspace.
- With a Pro workspace or higher, environment isolation can block cross-environment private network traffic.

For single-service apps, flat top-level `services`/`databases` is fine. Reach for the projects pattern when you have multiple services, need staging/production separation, or want environment-scoped env groups.

## Preview Environments

When a preview request is underspecified, first establish the desired generation mode, retention period, preview compute plans, and whether the user expects to exclude any resources.

Top-level `previews` controls Blueprint preview environments:

- **`previews.generation`**: `off` (default), `manual`, or `automatic`
- **`previews.expireAfterDays`**: Auto-delete preview stacks after N days

Do not confuse preview environments with service previews:

- A top-level `previews.generation: automatic` creates the Blueprint's preview environment, including the resources declared by that Blueprint.
- A service-level `previews.generation` controls that service's independent service previews. It does **not** override the Blueprint preview environment or exclude that service from it. It accepts only `manual` or `automatic`; omit it to disable independent service previews.
- Use `previews.plan` and `previews.numInstances` on compute services to size their preview-environment instances. Key Value and Postgres use `previewPlan` instead.

If a user needs a worker or another resource omitted entirely from Blueprint preview environments, explain that service-level preview generation is not an exclusion mechanism; the Blueprint or application architecture must account for that requirement. Other limitations include autoscaling behavior, `sync: false` variables, and database plan compatibility—see `references/preview-environments.md`.

## Immutable Fields

**CRITICAL:** Some fields cannot change after the resource is created. Edits may be rejected or require replacement resources.

### Services

- **`type`**: Cannot change (e.g. web → worker).
- **`runtime`**: Cannot change (e.g. node → docker).

### Databases

Cannot change after creation:

- `name` (logical Blueprint/database identifier in this context)
- `databaseName`
- `user`
- `region`
- `postgresMajorVersion`

Plan other fields (disk, HA, replicas) carefully up front; consult bex docs for fields that can scale vs require recreation.

## Key Fields (Quick Map)

| Area | Fields |
|------|--------|
| Plans | `plan`; compute-service `previews.plan`; Key Value/Postgres `previewPlan` |
| Build/run | `buildCommand`, `startCommand`, `preDeployCommand`, `rootDir` |
| Deploy | `autoDeployTrigger`: `commit`, `checksPass`, or `off` |
| Lifecycle | `maxShutdownDelaySeconds`: 1–300, default **30** |
| HTTP | `healthCheckPath`, `domains` |
| Storage | `disk` (`name`, `mountPath`, `sizeGB`) |
| Scale | `scaling` / `numInstances` (see references) |
| Monorepo | `buildFilter` (`paths`, `ignoredPaths`) |
| Docker | `dockerfilePath`, `dockerContext`, `dockerCommand`, `registryCredential`; prebuilt `image.url` / `image.creds` |

Deprecated names to avoid: `env` (use `runtime`), `redis` (use `keyvalue`), `autoDeploy` (use `autoDeployTrigger`), `previewsEnabled`, and `pullRequestPreviewsEnabled`. Use the appropriate top-level or service-level `previews.generation` replacement described in `references/common-mistakes.md`.

## References

| Document | Contents |
|----------|----------|
| `references/blueprints.md` | Current Blueprint specification, schema, validation, repository-sync, and safety workflow |
| `references/compute-plans.md` | Current compute-plan terminology, Plan ID discovery, availability, and change behavior |
| `references/environment-variables.md` | Current environment-variable documentation, secrets, groups, platform variables, and mutation safety |
| `references/persistent-disks.md` | Service-attached disk constraints, current documentation, sizing, snapshots, and Blueprint workflow |
| `references/private-networking.md` | Private-network scope, addresses, Blueprint references, discovery, ports, and isolation |
| `references/service-types.md` | Service-type selection, execution models, and Blueprint naming |
| `references/field-reference.md` | YAML fields by service type, database, groups, projects, previews, scaling, disk, static, Key Value |
| `references/wiring-patterns.md` | `fromDatabase` / `fromService` / `fromGroup` examples and combined wiring patterns |
| `references/common-mistakes.md` | Branch + previews, `buildFilter`, replicas, duplicates, preview plans, wiring mistakes |
| `references/preview-environments.md` | `previews.generation`, expiry, `previews.plan`, `previewPlan`, disks, PR workflow |

## Related Skills

- **bex-deploy** — Deploy flows, Blueprint vs direct create, MCP/deeplinks
- **bex-env-vars** — Env var strategy, secrets, Dashboard vs Blueprint
- **bex-docker** — Dockerfile-backed services and image runtime nuances

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
